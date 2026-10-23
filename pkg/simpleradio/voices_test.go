package simpleradio

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/dharmab/skyeye/pkg/simpleradio/types"
	"github.com/stretchr/testify/require"
)

// newTestConnection is a connected TCP pair: the client's end and the test server's end.
func newTestConnection(t *testing.T) (*net.TCPConn, *net.TCPConn) {
	t.Helper()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	accepted := make(chan *net.TCPConn, 1)
	go func() {
		conn, acceptErr := listener.AcceptTCP()
		if acceptErr == nil {
			accepted <- conn
		}
	}()
	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	conn, err := net.DialTCP("tcp", nil, addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	server := <-accepted
	t.Cleanup(func() { _ = server.Close() })
	return conn, server
}

func TestNewVoice(t *testing.T) {
	t.Parallel()
	owner := newSyncTestClient()
	v := owner.newVoice("Hatzerim Ground [BOT]", []RadioFrequency{testPackageFrequency, testPackageFrequency})
	require.Equal(t, "Hatzerim Ground [BOT]", v.Name())
	require.NotEqual(t, owner.clientInfo.GUID, v.clientInfo.GUID)
	require.Equal(t, owner.clientInfo.Coalition, v.clientInfo.Coalition)
	require.Len(t, v.allRadioInfo().Radios, 1, "a frequency named twice is one radio")
	require.True(t, v.allRadioInfo().Radios[0].IsSameFrequency(toRadio(testPackageFrequency)))
	require.Same(t, owner, v.owner)
	require.Same(t, owner.transmitLock(), v.transmitLock(), "a voice shares its owner's transmit lock")
	require.False(t, v.isReady())
}

// SRS routes a voice's calls to its owner too: the owner must not hear them or count the voice as
// a peer on frequency.
func TestOwnVoiceIsNotAPeer(t *testing.T) {
	t.Parallel()
	owner := newSyncTestClient()
	v := owner.newVoice("Vacuum [BOT]", []RadioFrequency{toFrequency(testRadio)})
	peer := testPeer(v.clientInfo.GUID, v.Name(), owner.clientInfo.Coalition, testRadio)
	owner.syncClient(peer)
	require.Equal(t, 1, owner.BotsOnFrequency(), "a voice not yet marked is an ordinary peer")

	owner.addOwnVoice(v.clientInfo.GUID)
	require.True(t, owner.isOwnVoice(v.clientInfo.GUID))
	require.Equal(t, 0, owner.BotsOnFrequency(), "marking a voice drops it from the peers")
	owner.syncClient(peer)
	_, known := owner.getPeerName(v.clientInfo.GUID)
	require.False(t, known)
	require.False(t, owner.isOwnVoice(testSelfGUID))
}

func TestSenderFor(t *testing.T) {
	t.Parallel()
	owner := newSyncTestClient()
	v := owner.newVoice("Blackjack [BOT]", []RadioFrequency{testPackageFrequency})
	stranger := newSyncTestClient().newVoice("Stranger [BOT]", nil)
	stranger.markReady()

	require.Same(t, owner, owner.senderFor(Transmission{}), "no voice named")
	require.Same(t, owner, owner.senderFor(Transmission{From: v}), "a voice not logged in yet")
	v.markReady()
	v.markReady()
	require.True(t, v.WaitReady(time.Millisecond))
	require.Same(t, v, owner.senderFor(Transmission{From: v}))
	require.Same(t, owner, owner.senderFor(Transmission{From: owner}))
	require.Same(t, owner, owner.senderFor(Transmission{From: stranger}), "another client's voice")
}

func TestWaitReadyTimesOut(t *testing.T) {
	t.Parallel()
	v := newSyncTestClient().newVoice("Hatzerim Tower [BOT]", nil)
	require.False(t, v.WaitReady(10*time.Millisecond))
}

func TestSetNameSendsUpdate(t *testing.T) {
	t.Parallel()
	conn, server := newTestConnection(t)
	c := newSyncTestClient()
	c.tcpConnection = conn
	require.NoError(t, c.SetName("Image [BOT]"))
	require.Equal(t, "Image [BOT]", c.Name())

	reader := bufio.NewReader(server)
	require.NoError(t, server.SetReadDeadline(time.Now().Add(2*time.Second)))
	line, err := reader.ReadBytes('\n')
	require.NoError(t, err)
	var message types.Message
	require.NoError(t, json.Unmarshal(line, &message))
	require.Equal(t, types.MessageUpdate, message.Type)
	require.NotNil(t, message.Client)
	require.Equal(t, "Image [BOT]", message.Client.Name)
	require.Equal(t, testSelfGUID, message.Client.GUID)

	require.NoError(t, c.SetName("Image [BOT]"))
	require.NoError(t, server.SetReadDeadline(time.Now().Add(200*time.Millisecond)))
	_, err = reader.ReadBytes('\n')
	require.Error(t, err, "the same name sends nothing")
}

// A voice's re-login waits for a call on the air, whichever client is sending it.
func TestVoiceResyncWaitsForOwnerTransmission(t *testing.T) {
	t.Parallel()
	conn, server := newTestConnection(t)
	owner := newSyncTestClient()
	v := owner.newVoice("Vacuum [BOT]", []RadioFrequency{toFrequency(testRadio)})
	v.tcpConnection = conn

	owner.txLock.Lock()
	done := make(chan error, 1)
	go func() { done <- v.AddListenRadios([]RadioFrequency{testPackageFrequency}) }()

	reader := bufio.NewReader(server)
	require.NoError(t, server.SetReadDeadline(time.Now().Add(300*time.Millisecond)))
	_, readErr := reader.ReadBytes('\n')
	require.Error(t, readErr, "voice resync was sent while a call held the lock")

	owner.txLock.Unlock()
	require.NoError(t, <-done)
	require.NoError(t, server.SetReadDeadline(time.Now().Add(2*time.Second)))
	line, readErr := reader.ReadBytes('\n')
	require.NoError(t, readErr)
	var message types.Message
	require.NoError(t, json.Unmarshal(line, &message))
	require.Equal(t, types.MessageSync, message.Type)
	require.NotNil(t, message.Client)
	require.Equal(t, "Vacuum [BOT]", message.Client.Name)
}
