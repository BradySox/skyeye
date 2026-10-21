package simpleradio

import (
	"testing"

	"github.com/dharmab/skyeye/pkg/simpleradio/types"
	"github.com/dharmab/skyeye/pkg/simpleradio/voice"
	"github.com/martinlindhe/unit"
	"github.com/stretchr/testify/require"
)

var testPackageFrequency = RadioFrequency{Frequency: 255.25 * unit.Megahertz, Modulation: types.ModulationAM}

// newListenTestClient is the sync test client with one listen radio on 255.250AM, added the way
// AddListenRadios adds it but without the server update.
func newListenTestClient() *Client {
	c := newSyncTestClient()
	c.receivers = map[types.Radio]*receiver{testRadio: {}}
	listen := toRadio(testPackageFrequency)
	c.clientInfo.RadioInfo.Radios = append(c.clientInfo.RadioInfo.Radios, listen)
	c.receivers[listen] = &receiver{}
	c.listenRadioCount = 1
	return c
}

func TestListenRadiosAreNotDefaultFrequencies(t *testing.T) {
	t.Parallel()
	c := newListenTestClient()
	require.Len(t, c.Frequencies(), 1)
	require.True(t, c.IsListenFrequency(testPackageFrequency))
	require.False(t, c.IsListenFrequency(toFrequency(testRadio)))
	require.Len(t, c.allRadioInfo().Radios, 2)
}

func TestTransmitFrequencies(t *testing.T) {
	t.Parallel()
	c := newListenTestClient()
	defaults := c.transmitFrequencies(Transmission{})
	require.Equal(t, []voice.Frequency{{Frequency: 251_000_000, Modulation: 0}}, defaults)
	targeted := c.transmitFrequencies(Transmission{Frequencies: []RadioFrequency{testPackageFrequency}})
	require.Equal(t, []voice.Frequency{{Frequency: 255_250_000, Modulation: 0}}, targeted)
}

func TestHeardFrequency(t *testing.T) {
	t.Parallel()
	c := newListenTestClient()
	heard := c.heardFrequency(voice.Packet{Frequencies: []voice.Frequency{{Frequency: 255_250_000}}})
	require.NotNil(t, heard)
	require.True(t, heard.IsSameFrequency(testPackageFrequency))
	require.Nil(t, c.heardFrequency(voice.Packet{Frequencies: []voice.Frequency{{Frequency: 300_000_000}}}))
}

func TestHumansOnAListenRadioDoNotCountAsOnFrequency(t *testing.T) {
	t.Parallel()
	c := newListenTestClient()
	listen := toRadio(testPackageFrequency)
	c.syncClient(testPeer("PeerGUID00000000000000", "Colt 1-1", c.clientInfo.Coalition, listen))
	require.Equal(t, 0, c.HumansOnFrequency())
	_, known := c.getPeerName("PeerGUID00000000000000")
	require.True(t, known)
}
