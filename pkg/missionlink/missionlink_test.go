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

func TestLinkTuneAndEmitter(t *testing.T) {
	t.Parallel()
	link, err := Listen("127.0.0.1:0")
	require.NoError(t, err)
	var tune Tune
	var emitter Emitter
	handlers := Handlers{
		OnTune:    func(m Tune) { tune = m },
		OnEmitter: func(m Emitter) { emitter = m },
	}
	link.handle([]byte(`{"type":"tune","frequencies":["262.000AM"]}`), nil, handlers)
	require.Equal(t, []string{"262.000AM"}, tune.Frequencies)
	link.handle([]byte(`{"type":"emitter","frequencies":["262.000AM","264.500AM"],"callsign":"Vacuum","label":"SA-6","lat":42.1,"lon":43.2}`), nil, handlers)
	require.Equal(t, Emitter{Type: TypeEmitter, Frequencies: []string{"262.000AM", "264.500AM"}, Callsign: "Vacuum", Label: "SA-6", Lat: 42.1, Lon: 43.2}, emitter)

	called := false
	ignore := Handlers{OnTune: func(Tune) { called = true }, OnEmitter: func(Emitter) { called = true }}
	link.handle([]byte(`{"type":"tune","frequencies":[]}`), nil, ignore)
	link.handle([]byte(`{"type":"emitter","frequencies":["262.000AM"],"label":""}`), nil, ignore)
	require.False(t, called)
}

// Each script gets the replies for what it announced: package requests to the pkgcontrol socket,
// ATC requests to the ATC socket.
func TestLinkRoutesRepliesByOwner(t *testing.T) {
	t.Parallel()
	link, err := Listen("127.0.0.1:0")
	require.NoError(t, err)
	atcCh := make(chan ATC, 1)
	packagesCh := make(chan Packages, 1)
	go link.Run(t.Context(), Handlers{
		OnATC:      func(a ATC) { atcCh <- a },
		OnPackages: func(p Packages) { packagesCh <- p },
	})
	skyeye, ok := link.Addr().(*net.UDPAddr)
	require.True(t, ok)

	pkgcontrol, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer pkgcontrol.Close()
	atc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer atc.Close()

	require.ErrorContains(t, link.SendATC(ATCRequest{Kind: "taxi"}), "no mission")

	_, err = pkgcontrol.WriteToUDP([]byte(`{"type":"packages","controllers":["Anvil"],"packages":[]}`), skyeye)
	require.NoError(t, err)
	select {
	case <-packagesCh:
	case <-time.After(2 * time.Second):
		t.Fatal("no packages message")
	}
	_, err = atc.WriteToUDP([]byte(`{"type":"atc","fields":[{"name":"Kobuleti","positions":[{"position":"ground","frequency":"260.000AM"},{"position":"tower","frequency":"262.000AM"}]}]}`), skyeye)
	require.NoError(t, err)
	select {
	case a := <-atcCh:
		require.Equal(t, []ATCField{{Name: "Kobuleti", Positions: []ATCPosition{
			{Position: "ground", Frequency: "260.000AM"},
			{Position: "tower", Frequency: "262.000AM"},
		}}}, a.Fields)
	case <-time.After(2 * time.Second):
		t.Fatal("no atc message")
	}

	read := func(conn *net.UDPConn) map[string]any {
		t.Helper()
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
		buf := make([]byte, 2048)
		n, _, err := conn.ReadFromUDP(buf)
		require.NoError(t, err)
		var message map[string]any
		require.NoError(t, json.Unmarshal(buf[:n], &message))
		return message
	}
	require.NoError(t, link.SendATC(ATCRequest{Kind: "taxi", Callsign: "viper 1 1", Position: "ground", Numbers: "", Frequency: "260.000AM"}))
	require.Equal(t, TypeATCRequest, read(atc)["type"])
	require.NoError(t, link.Send(Request{Kind: "checkin", Callsign: "viper 1 1", Controller: "anvil"}))
	require.Equal(t, TypeRequest, read(pkgcontrol)["type"])
}
