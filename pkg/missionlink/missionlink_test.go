package missionlink

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLinkRoundTrip(t *testing.T) {
	t.Parallel()
	link, err := Listen("127.0.0.1:0")
	require.NoError(t, err)

	packagesCh := make(chan Packages, 1)
	sayCh := make(chan Say, 1)
	ctx := t.Context()
	go link.Run(ctx, Handlers{
		OnPackages: func(p Packages) { packagesCh <- p },
		OnSay:      func(s Say) { sayCh <- s },
	})

	mission, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer mission.Close()
	skyeye, ok := link.Addr().(*net.UDPAddr)
	require.True(t, ok)

	require.ErrorContains(t, link.Send(Request{Kind: "checkin"}), "no mission")

	_, err = mission.WriteToUDP([]byte(`{"type":"packages","controllers":["Darkstar"],"packages":[{"name":"Colt","frequency":"251.250AM"}]}`), skyeye)
	require.NoError(t, err)
	select {
	case p := <-packagesCh:
		require.Equal(t, []string{"Darkstar"}, p.Controllers)
		require.Equal(t, []Package{{Name: "Colt", Frequency: "251.250AM"}}, p.Packages)
	case <-time.After(2 * time.Second):
		t.Fatal("no packages message")
	}

	_, err = mission.WriteToUDP([]byte(`{"type":"say","frequency":"251.250AM","text":"Package Colt, push, push."}`), skyeye)
	require.NoError(t, err)
	select {
	case s := <-sayCh:
		require.Equal(t, "251.250AM", s.Frequency)
		require.Equal(t, "Package Colt, push, push.", s.Text)
	case <-time.After(2 * time.Second):
		t.Fatal("no say message")
	}

	require.NoError(t, link.Send(Request{Kind: "checkin", Callsign: "colt 1 1", Controller: "darkstar", Frequency: "251.250AM"}))
	require.NoError(t, mission.SetReadDeadline(time.Now().Add(2*time.Second)))
	buf := make([]byte, 2048)
	n, _, err := mission.ReadFromUDP(buf)
	require.NoError(t, err)
	var request Request
	require.NoError(t, json.Unmarshal(buf[:n], &request))
	require.Equal(t, Request{Type: TypeRequest, Kind: "checkin", Callsign: "colt 1 1", Controller: "darkstar", Frequency: "251.250AM"}, request)
}

func TestLinkIgnoresGarbage(t *testing.T) {
	t.Parallel()
	link, err := Listen("127.0.0.1:0")
	require.NoError(t, err)
	called := false
	link.handle([]byte("not json"), nil, Handlers{OnSay: func(Say) { called = true }})
	link.handle([]byte(`{"type":"say","text":""}`), nil, Handlers{OnSay: func(Say) { called = true }})
	require.False(t, called)
}
