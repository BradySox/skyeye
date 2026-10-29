// Package radiorecord writes the radio calls heard and spoken on chosen frequencies to one WAV
// file per frequency, for playback alongside a Tacview recording. Each file's name carries the
// mission time its first sample plays at, as YYYYMMDDTHHMMSSZ, which Tacview reads to line the
// audio up with the replay. Calls are placed by mission time, with silence between them.
//
//nolint:gosec // WAV samples and sizes are fixed-width 16- and 32-bit fields
package radiorecord

import (
	"encoding/binary"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SampleRate is the rate of the PCM the recorder takes and writes: SkyEye's wideband audio.
const SampleRate = 16000

const (
	bitsPerSample = 16
	headerSize    = 44
	stampLayout   = "20060102T150405Z"
	// maxGap starts a new file when a call lands this far outside its file: a new mission, or a
	// mission clock that jumped.
	maxGap = 6 * time.Hour
)

// Recorder writes one WAV file per frequency. It is safe for concurrent use.
type Recorder struct {
	dir string
	// clock is the mission time now; zero when it is not known yet.
	clock func() time.Time
	lock  sync.Mutex
	// labels name each recorded frequency in its file name, keyed by frequency.
	labels map[string]string
	tracks map[string]*track
}

// New makes a recorder that writes to dir. clock returns the mission time now.
func New(dir string, clock func() time.Time) *Recorder {
	return &Recorder{dir: dir, clock: clock, labels: map[string]string{}, tracks: map[string]*track{}}
}

// SetFrequencies replaces the frequencies to record, keyed by frequency (such as "251.000AM")
// with a label for the file name. Files of frequencies no longer listed, or whose label
// changed, are closed.
func (r *Recorder) SetFrequencies(labels map[string]string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	for key, t := range r.tracks {
		if label, ok := labels[key]; !ok || label != r.labels[key] {
			t.close()
			delete(r.tracks, key)
		}
	}
	r.labels = make(map[string]string, len(labels))
	maps.Copy(r.labels, labels)
}

// IsRecorded is true if the frequency is one the recorder writes.
func (r *Recorder) IsRecorded(frequency string) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	_, ok := r.labels[frequency]
	return ok
}

// Record writes a call heard or spoken on frequency that started at the given wall time.
func (r *Recorder) Record(frequency string, started time.Time, pcm []float32) {
	if len(pcm) == 0 {
		return
	}
	now := r.clock()
	if now.IsZero() {
		log.Debug().Str("frequency", frequency).Msg("not recording a call before the mission clock is known")
		return
	}
	at := now.Add(-time.Since(started))
	r.lock.Lock()
	defer r.lock.Unlock()
	label, ok := r.labels[frequency]
	if !ok {
		return
	}
	t := r.tracks[frequency]
	if t != nil && (at.Before(t.start.Add(-time.Minute)) || at.After(t.start.Add(maxGap))) {
		t.close()
		t = nil
	}
	if t == nil {
		var err error
		t, err = openTrack(r.dir, label, frequency, at)
		if err != nil {
			log.Error().Err(err).Str("frequency", frequency).Msg("failed to open radio recording")
			return
		}
		r.tracks[frequency] = t
	}
	if err := t.write(at, pcm); err != nil {
		log.Error().Err(err).Str("frequency", frequency).Msg("failed to write radio recording")
	}
}

// Close closes every file.
func (r *Recorder) Close() {
	r.lock.Lock()
	defer r.lock.Unlock()
	for key, t := range r.tracks {
		t.close()
		delete(r.tracks, key)
	}
}

// FileName is the recording's file name for a label, a frequency and the mission time its
// first sample plays at.
func FileName(label, frequency string, start time.Time) string {
	name := frequency
	if label != "" {
		name = label + " " + frequency
	}
	return fmt.Sprintf("SkyEye_%s_%s.wav", sanitize(name), start.UTC().Format(stampLayout))
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < ' ' {
			return '_'
		}
		return r
	}, s)
}

// track is one frequency's WAV file.
type track struct {
	file *os.File
	// start is the mission time of the file's first sample, a whole second.
	start time.Time
	// samples written so far.
	samples int64
}

func openTrack(dir, label, frequency string, at time.Time) (*track, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to make recording folder: %w", err)
	}
	start := at.UTC().Truncate(time.Second)
	path := filepath.Join(dir, FileName(label, frequency, start))
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create recording: %w", err)
	}
	t := &track{file: f, start: start}
	if err := t.writeHeader(); err != nil {
		_ = f.Close()
		return nil, err
	}
	log.Info().Str("path", path).Msg("recording radio frequency")
	return t, nil
}

// write places pcm at mission time at. A call that overlaps the one before is mixed into it.
func (t *track) write(at time.Time, pcm []float32) error {
	offset := int64(math.Round(at.Sub(t.start).Seconds() * SampleRate))
	if offset < 0 {
		pcm = pcm[min(int64(len(pcm)), -offset):]
		offset = 0
	}
	if len(pcm) == 0 {
		return nil
	}
	for t.samples < offset {
		n := min(offset-t.samples, SampleRate*60)
		if err := t.writeAt(t.samples, make([]int16, n)); err != nil {
			return err
		}
		t.samples += n
	}
	out := make([]int16, len(pcm))
	if overlap := min(t.samples-offset, int64(len(pcm))); overlap > 0 {
		existing, err := t.readAt(offset, int(overlap))
		if err != nil {
			return err
		}
		copy(out, existing)
	}
	for i, s := range pcm {
		out[i] = clamp(float64(out[i]) + float64(s)*math.MaxInt16)
	}
	if err := t.writeAt(offset, out); err != nil {
		return err
	}
	t.samples = max(t.samples, offset+int64(len(out)))
	// The header carries the length, so the file stays playable if SkyEye stops without closing it.
	return t.writeHeader()
}

func clamp(v float64) int16 {
	return int16(max(math.MinInt16, min(math.MaxInt16, math.Round(v))))
}

func (t *track) writeAt(sample int64, data []int16) error {
	b := make([]byte, 2*len(data))
	for i, s := range data {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(s))
	}
	_, err := t.file.WriteAt(b, headerSize+2*sample)
	return err
}

func (t *track) readAt(sample int64, n int) ([]int16, error) {
	b := make([]byte, 2*n)
	if _, err := t.file.ReadAt(b, headerSize+2*sample); err != nil {
		return nil, err
	}
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[2*i:]))
	}
	return out, nil
}

func (t *track) writeHeader() error {
	dataSize := uint32(2 * t.samples)
	h := make([]byte, headerSize)
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], 36+dataSize)
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1) // PCM
	binary.LittleEndian.PutUint16(h[22:], 1) // mono
	binary.LittleEndian.PutUint32(h[24:], SampleRate)
	binary.LittleEndian.PutUint32(h[28:], SampleRate*bitsPerSample/8)
	binary.LittleEndian.PutUint16(h[32:], bitsPerSample/8)
	binary.LittleEndian.PutUint16(h[34:], bitsPerSample)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], dataSize)
	_, err := t.file.WriteAt(h, 0)
	return err
}

func (t *track) close() {
	if err := t.file.Close(); err != nil {
		log.Error().Err(err).Str("path", t.file.Name()).Msg("failed to close radio recording")
	}
}
