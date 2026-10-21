package simpleradio

import (
	"bufio"
	"encoding/json"
	"net"
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

// An SRS server ignores RADIO_UPDATE from an External AWACS Mode client, so listen radios added
// after connecting must reach it in a sync and an EAM login.
func TestAddListenRadiosResyncsWithServer(t *testing.T) {
	t.Parallel()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer listener.Close()
	accepted := make(chan *net.TCPConn, 1)
	go func() {
		conn, acceptErr := listener.AcceptTCP()
		if acceptErr == nil {
			accepted <- conn
		}
	}()
	conn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	require.NoError(t, err)
	defer conn.Close()
	server := <-accepted
	defer server.Close()

	c := newSyncTestClient()
	c.receivers = map[types.Radio]*receiver{testRadio: {}}
	c.tcpConnection = conn
	c.externalAWACSModePassword = "hunter2"
	require.NoError(t, c.AddListenRadios([]RadioFrequency{testPackageFrequency}))

	reader := bufio.NewReader(server)
	sent := make([]types.MessageType, 0, 2)
	for range 2 {
		line, readErr := reader.ReadBytes('\n')
		require.NoError(t, readErr)
		var message types.Message
		require.NoError(t, json.Unmarshal(line, &message))
		require.NotNil(t, message.Client)
		require.Len(t, message.Client.RadioInfo.Radios, 2)
		require.True(t, message.Client.RadioInfo.Radios[1].IsSameFrequency(toRadio(testPackageFrequency)))
		sent = append(sent, message.Type)
	}
	require.Equal(t, []types.MessageType{types.MessageSync, types.MessageExternalAWACSModePassword}, sent)
}
