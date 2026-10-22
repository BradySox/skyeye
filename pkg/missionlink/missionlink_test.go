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
	link.handle([]byte(`{"type":"emitter","frequencies":["262.000AM","264.500AM"],"callsign":"Vacuum","label":"SA-6","status":"awake","fix":"tight","lat":42.1,"lon":43.2}`), nil, handlers)
	require.Equal(t, Emitter{Type: TypeEmitter, Frequencies: []string{"262.000AM", "264.500AM"}, Callsign: "Vacuum", Label: "SA-6", Status: "awake", Fix: "tight", Lat: 42.1, Lon: 43.2}, emitter)

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
	_, err = atc.WriteToUDP([]byte(`{"type":"atc","fields":[{"name":"Kobuleti","positions":[{"position":"ground","frequency":"260.000AM"},{"position":"tower","frequency":"262.000AM"}]}],"callsigns":["Uzi 2","Colt 1"]}`), skyeye)
	require.NoError(t, err)
	select {
	case a := <-atcCh:
		require.Equal(t, []ATCField{{Name: "Kobuleti", Positions: []ATCPosition{
			{Position: "ground", Frequency: "260.000AM"},
			{Position: "tower", Frequency: "262.000AM"},
		}}}, a.Fields)
		require.Equal(t, []string{"Uzi 2", "Colt 1"}, a.Callsigns)
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

// A tune that names a speaker gets the ELINT requests for it; a plain tune does not.
func TestLinkSendsELINTRequestsToTheSpeaker(t *testing.T) {
	t.Parallel()
	link, err := Listen("127.0.0.1:0")
	require.NoError(t, err)
	tuneCh := make(chan Tune, 2)
	go link.Run(t.Context(), Handlers{OnTune: func(m Tune) { tuneCh <- m }})
	skyeye, ok := link.Addr().(*net.UDPAddr)
	require.True(t, ok)
	vacuum, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer vacuum.Close()

	_, err = vacuum.WriteToUDP([]byte(`{"type":"tune","frequencies":["262.000AM"]}`), skyeye)
	require.NoError(t, err)
	<-tuneCh
	require.ErrorContains(t, link.SendELINT(ELINTRequest{Kind: "emitters"}), "no mission")

	_, err = vacuum.WriteToUDP([]byte(`{"type":"tune","frequencies":["262.000AM","264.500AM"],"callsign":"Vacuum","frequency":"262.000AM"}`), skyeye)
	require.NoError(t, err)
	select {
	case tune := <-tuneCh:
		require.Equal(t, Tune{Type: TypeTune, Frequencies: []string{"262.000AM", "264.500AM"}, Callsign: "Vacuum", Frequency: "262.000AM"}, tune)
	case <-time.After(2 * time.Second):
		t.Fatal("no tune message")
	}

	require.NoError(t, link.SendELINT(ELINTRequest{Kind: "mud", Callsign: "ford 9", Speaker: "vacuum", Bearing: "270", Frequency: "262.000AM"}))
	require.NoError(t, vacuum.SetReadDeadline(time.Now().Add(2*time.Second)))
	buf := make([]byte, 2048)
	n, _, err := vacuum.ReadFromUDP(buf)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"elintRequest","kind":"mud","callsign":"ford 9","speaker":"vacuum","bearing":"270","frequency":"262.000AM"}`, string(buf[:n]))
}

// The mission names each player's flight callsign, so the GCI finds "Ford 9-1" when the player is
// "Flash".
func TestPackagesCarryPilots(t *testing.T) {
	t.Parallel()
	var packages Packages
	require.NoError(t, json.Unmarshal([]byte(`{"type":"packages","controllers":[],"packages":[],"pilots":[{"name":"Flash","callsign":"Ford 9-1"}]}`), &packages))
	require.Equal(t, []Pilot{{Name: "Flash", Callsign: "Ford 9-1"}}, packages.Pilots)
}
