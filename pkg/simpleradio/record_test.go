package simpleradio

import (
	"sync"
	"testing"
	"time"

	"github.com/dharmab/skyeye/pkg/simpleradio/types"
	"github.com/dharmab/skyeye/pkg/simpleradio/voice"
	"github.com/stretchr/testify/require"
)

type fakeRecorder struct {
	lock  sync.Mutex
	calls []string
}

func (r *fakeRecorder) Record(frequency string, _ time.Time, _ []float32) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.calls = append(r.calls, frequency)
}

func newRecordTestClient(recordOnly bool) (*Client, *fakeRecorder) {
	c := newListenTestClient()
	recorder := &fakeRecorder{}
	c.SetRecorder(recorder)
	c.recordChan = make(chan recording, 4)
	c.recorded = map[types.Radio]bool{toRadio(testPackageFrequency): recordOnly}
	c.clients = map[types.GUID]types.ClientInfo{
		"allows": {GUID: "allows", AllowRecording: true},
		"denies": {GUID: "denies", AllowRecording: false},
	}
	return c, recorder
}

func heard(origin types.GUID) *receiver {
	return &receiver{buffer: []voice.Packet{{}}, origin: origin, started: time.Now()}
}

func TestHeardCallsAreQueuedForTheRecorder(t *testing.T) {
	t.Parallel()
	c, _ := newRecordTestClient(false)
	listen := toRadio(testPackageFrequency)
	require.False(t, c.queueRecording(listen, heard("allows")))
	require.Len(t, c.recordChan, 1)
	rec := <-c.recordChan
	require.Equal(t, testPackageFrequency.String(), rec.frequency)

	// A player who turned SRS recording off is left out, and so is a sender SkyEye does not know.
	c.queueRecording(listen, heard("denies"))
	c.queueRecording(listen, heard("stranger"))
	require.Empty(t, c.recordChan)

	// A radio that is not recorded queues nothing.
	require.False(t, c.queueRecording(testRadio, heard("allows")))
	require.Empty(t, c.recordChan)
}

func TestRecordOnlyRadiosAreNotAnswered(t *testing.T) {
	t.Parallel()
	c, _ := newRecordTestClient(true)
	require.True(t, c.queueRecording(toRadio(testPackageFrequency), heard("allows")))
	require.Len(t, c.recordChan, 1)
}

func TestSpokenCallsAreRecordedOnRecordedFrequencies(t *testing.T) {
	t.Parallel()
	c, recorder := newRecordTestClient(false)
	listen := toRadio(testPackageFrequency)
	out := outgoing{
		audio: []float32{0.5},
		frequencies: []voice.Frequency{
			{Frequency: listen.Frequency, Modulation: byte(listen.Modulation)},
			{Frequency: testRadio.Frequency, Modulation: byte(testRadio.Modulation)},
		},
	}
	c.recordSpoken(out, time.Now())
	require.Equal(t, []string{testPackageFrequency.String()}, recorder.calls)
}
