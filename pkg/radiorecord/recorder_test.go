//nolint:gosec // WAV samples and sizes are fixed-width 16- and 32-bit fields
package radiorecord

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func tone(n int, v float32) []float32 {
	pcm := make([]float32, n)
	for i := range pcm {
		pcm[i] = v
	}
	return pcm
}

func readWAV(t *testing.T, path string) []int16 {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "RIFF", string(b[0:4]))
	require.Equal(t, "WAVE", string(b[8:12]))
	require.Equal(t, uint32(SampleRate), binary.LittleEndian.Uint32(b[24:]))
	size := binary.LittleEndian.Uint32(b[40:])
	require.Equal(t, uint32(len(b)-headerSize), size)
	out := make([]int16, size/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[headerSize+2*i:]))
	}
	return out
}

func TestFileNameCarriesTheMissionTime(t *testing.T) {
	t.Parallel()
	start := time.Date(1991, 1, 17, 3, 4, 5, 0, time.UTC)
	require.Equal(t, "SkyEye_Kobuleti Ground 270.450AM_19910117T030405Z.wav", FileName("Kobuleti Ground", "270.450AM", start))
	require.Equal(t, "SkyEye_251.000AM_19910117T030405Z.wav", FileName("", "251.000AM", start))
	require.Equal(t, "SkyEye_A_B 1AM_19910117T030405Z.wav", FileName("A/B", "1AM", start))
}

func TestRecordPlacesCallsByMissionTime(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mission := time.Date(1991, 1, 17, 3, 0, 0, 500_000_000, time.UTC)
	wall := time.Now()
	clock := func() time.Time { return mission.Add(time.Since(wall)) }
	r := New(dir, clock)
	r.SetFrequencies(map[string]string{"251.000AM": "AWACS"})
	require.True(t, r.IsRecorded("251.000AM"))
	require.False(t, r.IsRecorded("262.000AM"))

	// A call on a frequency not recorded writes nothing.
	r.Record("262.000AM", wall, tone(100, 0.5))
	// The first call starts the file at the whole mission second before it.
	r.Record("251.000AM", wall, tone(SampleRate/10, 0.5))
	// A call two seconds later lands two seconds in, with silence before it.
	r.Record("251.000AM", wall.Add(2*time.Second), tone(SampleRate/10, 0.25))
	r.Close()

	files, err := filepath.Glob(filepath.Join(dir, "*.wav"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "SkyEye_AWACS 251.000AM_19910117T030000Z.wav", filepath.Base(files[0]))

	samples := readWAV(t, files[0])
	first := SampleRate / 2
	second := first + 2*SampleRate
	// Wall time passes between the calls, so allow a few milliseconds either way.
	const slack = SampleRate / 50
	require.InDelta(t, second+SampleRate/10, len(samples), slack)
	require.Equal(t, int16(0), samples[first-slack])
	require.InDelta(t, 16384, samples[first+slack], 1)
	require.Equal(t, int16(0), samples[second-slack])
	require.InDelta(t, 8192, samples[second+slack], 1)
}

func TestOverlappingCallsMix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fixed := time.Date(2026, 10, 29, 16, 40, 0, 0, time.UTC)
	wall := time.Now()
	r := New(dir, func() time.Time { return fixed.Add(time.Since(wall)) })
	r.SetFrequencies(map[string]string{"264.500AM": "STRIKE"})
	r.Record("264.500AM", wall, tone(SampleRate, 0.25))
	r.Record("264.500AM", wall, tone(SampleRate/2, 0.25))
	r.Close()
	files, err := filepath.Glob(filepath.Join(dir, "*.wav"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	samples := readWAV(t, files[0])
	require.InDelta(t, 16384, samples[SampleRate/4], 2)
	require.InDelta(t, 8192, samples[3*SampleRate/4], 2)
}

func TestNoClockNoRecording(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := New(dir, func() time.Time { return time.Time{} })
	r.SetFrequencies(map[string]string{"251.000AM": "AWACS"})
	r.Record("251.000AM", time.Now(), tone(100, 0.5))
	r.Close()
	files, err := filepath.Glob(filepath.Join(dir, "*.wav"))
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestNewMissionStartsANewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 10, 29, 16, 40, 0, 0, time.UTC)
	r := New(dir, func() time.Time { return now })
	r.SetFrequencies(map[string]string{"251.000AM": "AWACS"})
	r.Record("251.000AM", time.Now(), tone(100, 0.5))
	now = time.Date(1991, 1, 17, 3, 0, 0, 0, time.UTC)
	r.Record("251.000AM", time.Now(), tone(100, 0.5))
	r.Close()
	files, err := filepath.Glob(filepath.Join(dir, "*.wav"))
	require.NoError(t, err)
	require.Len(t, files, 2)
}
