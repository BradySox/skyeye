package simpleradio

import (
	"context"
	"time"

	"github.com/dharmab/skyeye/pkg/pcm/rate"
	"github.com/dharmab/skyeye/pkg/simpleradio/types"
	"github.com/dharmab/skyeye/pkg/simpleradio/voice"
	"github.com/rs/zerolog/log"
	"gopkg.in/hraban/opus.v2"
)

// Recorder takes every call heard or spoken on the frequencies passed to RecordRadios, as F32LE
// PCM with the wall time it started.
type Recorder interface {
	Record(frequency string, started time.Time, pcm []float32)
}

// recording is a heard call on its way to the recorder.
type recording struct {
	frequency string
	started   time.Time
	packets   []voice.Packet
}

// SetRecorder sets the recorder that RecordRadios feeds. Call it before Run.
func (c *Client) SetRecorder(recorder Recorder) {
	c.recorder = recorder
}

// RecordRadios tunes the frequencies as listen radios and records every call heard or spoken on
// them, replacing the frequencies recorded before. On a record-only frequency calls are recorded
// and never passed to Receive, so nothing heard there is answered.
func (c *Client) RecordRadios(frequencies []RadioFrequency, recordOnly []bool) error {
	if err := c.AddListenRadios(frequencies); err != nil {
		return err
	}
	c.radioLock.Lock()
	defer c.radioLock.Unlock()
	c.recorded = make(map[types.Radio]bool, len(frequencies))
	for i, frequency := range frequencies {
		c.recorded[toRadio(frequency)] = i < len(recordOnly) && recordOnly[i]
	}
	return nil
}

// recordedRadio is the recorded radio matching radio, and whether it is record-only. Call it with
// radioLock held.
func (c *Client) recordedRadio(radio types.Radio) (types.Radio, bool, bool) {
	for recorded, only := range c.recorded {
		if recorded.IsSameFrequency(radio) {
			return recorded, only, true
		}
	}
	return types.Radio{}, false, false
}

// allowsRecording is true for a peer that left SRS's recording consent on.
func (c *Client) allowsRecording(guid types.GUID) bool {
	c.clientsLock.RLock()
	defer c.clientsLock.RUnlock()
	info, ok := c.clients[guid]
	return ok && info.AllowRecording
}

// queueRecording sends a heard call to the recorder when its radio is recorded and its sender
// allows recording. It is true when the radio is record-only. Call it with radioLock held.
func (c *Client) queueRecording(radio types.Radio, r *receiver) bool {
	recorded, only, ok := c.recordedRadio(radio)
	if !ok || c.recorder == nil {
		return false
	}
	if c.allowsRecording(r.origin) {
		packets := make([]voice.Packet, len(r.buffer))
		copy(packets, r.buffer)
		select {
		case c.recordChan <- recording{frequency: toFrequency(recorded).String(), started: r.started, packets: packets}:
		default:
			log.Warn().Msg("radio recorder is behind; dropping a call from the recording")
		}
	}
	return only
}

// recordVoice decodes heard calls queued for the recorder.
func (c *Client) recordVoice(ctx context.Context) {
	for {
		select {
		case rec := <-c.recordChan:
			decoder, err := opus.NewDecoder(int(rate.Wideband.Hertz()), channels)
			if err != nil {
				log.Error().Err(err).Msg("failed to create Opus decoder for the recording")
				continue
			}
			pcm := make([]float32, 0, len(rec.packets)*int(frameSize))
			for _, packet := range rec.packets {
				frame, err := decodeFrame(decoder, packet.AudioBytes)
				if err == nil {
					pcm = append(pcm, frame...)
				}
			}
			c.recorder.Record(rec.frequency, rec.started, pcm)
		case <-ctx.Done():
			return
		}
	}
}

// recordSpoken records a call this client sent, on each of its frequencies that is recorded.
func (c *Client) recordSpoken(out outgoing, started time.Time) {
	if c.recorder == nil || len(out.audio) == 0 {
		return
	}
	c.radioLock.RLock()
	var frequencies []string
	for _, f := range out.frequencies {
		radio := types.Radio{Frequency: f.Frequency, Modulation: types.Modulation(f.Modulation)}
		if recorded, _, ok := c.recordedRadio(radio); ok {
			frequencies = append(frequencies, toFrequency(recorded).String())
		}
	}
	c.radioLock.RUnlock()
	for _, frequency := range frequencies {
		c.recorder.Record(frequency, started, out.audio)
	}
}
