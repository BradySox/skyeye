// Package missionlink is a local UDP link between SkyEye and a mission script running a package
// controller (for example, DCS Retribution's RetLab package controller).
//
// The mission sends SkyEye its packages (frequencies and controller callsigns), its airfield
// controllers and lines to speak. SkyEye sends the mission the requests it heard. Messages are one
// JSON object per UDP datagram. Several mission scripts may share the link, each from its own
// socket: package requests go to the address the last packages message came from, ATC requests to
// the address the last atc message came from.
package missionlink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/rs/zerolog/log"
)

// Message types.
const (
	TypePackages = "packages"
	TypeSay      = "say"
	TypeRequest  = "request"
	TypeTune     = "tune"
	TypeEmitter  = "emitter"
	TypeATC      = "atc"
	// TypeATCRequest is an ATC call SkyEye heard, sent to the mission.
	TypeATCRequest = "atcRequest"
)

// maxDatagram is the largest message accepted.
const maxDatagram = 64 * 1024

// Package is one package the mission controls.
type Package struct {
	// Name of the package, as the controller says it.
	Name string `json:"name"`
	// Frequency the package talks on, such as "251.250AM".
	Frequency string `json:"frequency"`
}

// Packages tells SkyEye the controller callsigns and package frequencies to listen for.
type Packages struct {
	Type        string    `json:"type"`
	Controllers []string  `json:"controllers"`
	Packages    []Package `json:"packages"`
	// Strike is the common strike frequency, such as "264.500AM". SkyEye joins it as the
	// GCI, but only for calls that matter there: answers to calls heard on it, threat
	// calls and merges.
	Strike string `json:"strike,omitempty"`
	// GCIVoice is "feminine" or "masculine": SkyEye's own voice for this mission.
	GCIVoice string `json:"gciVoice,omitempty"`
	// GCICallsign is SkyEye's own callsign for this mission, such as "Chainsaw".
	GCICallsign string `json:"gciCallsign,omitempty"`
	// LooseCalls lets a package call on the strike frequency name only the pilot, for squadrons
	// that do not fly by the book.
	LooseCalls bool `json:"looseCalls,omitempty"`
	// StrikeThreatsOnly keeps the GCI on strike to threat calls: no merges, and answers to calls
	// heard on strike go out on the GCI's own frequencies.
	StrikeThreatsOnly bool `json:"strikeThreatsOnly,omitempty"`
	// Pilots are the players in the mission's flights, so the GCI finds a player by the flight
	// callsign they say when their in-game name is something else.
	Pilots []Pilot `json:"pilots,omitempty"`
}

// Pilot is a player and the flight callsign they fly.
type Pilot struct {
	// Name is the player's in-game name, such as "Flash".
	Name string `json:"name"`
	// Callsign is the seat they fly, such as "Ford 9-1".
	Callsign string `json:"callsign"`
}

// Say asks SkyEye to speak a line on a frequency.
type Say struct {
	Type      string `json:"type"`
	Frequency string `json:"frequency"`
	Text      string `json:"text"`
	// Speech is what is spoken, when it differs from the text: "squawk four one zero one" for
	// "squawk 4101", which Piper would read as a number. Empty speaks the text.
	Speech string `json:"speech,omitempty"`
	// Voice is "feminine" or "masculine"; empty uses SkyEye's own voice.
	Voice string `json:"voice,omitempty"`
}

// Tune asks SkyEye to join more frequencies to speak on, leaving the package controllers alone.
type Tune struct {
	Type        string   `json:"type"`
	Frequencies []string `json:"frequencies"`
}

// Emitter asks SkyEye to report a radiating ground emitter with a bullseye call. SkyEye works out
// the bullseye itself, so the bearing is magnetic and matches its own calls.
type Emitter struct {
	Type string `json:"type"`
	// Frequencies to speak on, such as "262.000AM".
	Frequencies []string `json:"frequencies"`
	// Callsign of the speaker, such as "Vacuum".
	Callsign string `json:"callsign"`
	// Voice of the speaker; empty uses SkyEye's own voice.
	Voice string `json:"voice,omitempty"`
	// Label of the emitter as spoken, such as "SA-6".
	Label string  `json:"label"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
}

// ATCPosition is one controller position at an airfield.
type ATCPosition struct {
	// Position is "ground", "tower", "departure", "approach", "arrival" or "emergency".
	Position string `json:"position"`
	// Frequency the position talks on, such as "270.450AM".
	Frequency string `json:"frequency"`
}

// ATCField is an airfield the mission runs ATC for.
type ATCField struct {
	Name      string        `json:"name"`
	Positions []ATCPosition `json:"positions"`
}

// ATC tells SkyEye the airfield frequencies to listen on. Every call heard on one is parsed as an
// ATC call and passed to the mission, which answers with say messages.
type ATC struct {
	Type   string     `json:"type"`
	Fields []ATCField `json:"fields"`
	// Discipline is the mission's radio discipline: "strict", "relaxed" or "casual". Casual
	// parses calls loosely.
	Discipline string `json:"discipline,omitempty"`
	// Callsigns are the flights the mission runs ATC for, such as "Uzi 2". SkyEye only uses them
	// to steer speech recognition toward them.
	Callsigns []string `json:"callsigns,omitempty"`
}

// ATCRequest is an ATC call SkyEye heard, sent to the mission. Numbers is space-separated, so the
// mission script can read every field as a flat string.
type ATCRequest struct {
	Type      string `json:"type"`
	Kind      string `json:"kind"`
	Callsign  string `json:"callsign"`
	Position  string `json:"position,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Numbers   string `json:"numbers"`
	Text      string `json:"text"`
	Frequency string `json:"frequency"`
}

// Request is a package request SkyEye heard, sent to the mission.
type Request struct {
	Type       string `json:"type"`
	Kind       string `json:"kind"`
	Callsign   string `json:"callsign"`
	Controller string `json:"controller"`
	Frequency  string `json:"frequency,omitempty"`
}

// Handlers receive the mission's messages.
type Handlers struct {
	OnPackages func(Packages)
	OnSay      func(Say)
	OnTune     func(Tune)
	OnEmitter  func(Emitter)
	OnATC      func(ATC)
}

// Link is the UDP link.
type Link struct {
	conn *net.UDPConn
	// peers are the mission scripts by the message type they announce with.
	peers    map[string]*net.UDPAddr
	peerLock sync.RWMutex
}

// Listen opens the link on a local UDP address such as "127.0.0.1:42690".
func Listen(address string) (*Link, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve mission link address %q: %w", address, err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on mission link address %q: %w", address, err)
	}
	return &Link{conn: conn, peers: map[string]*net.UDPAddr{}}, nil
}

// Addr is the address the link listens on.
func (l *Link) Addr() net.Addr {
	return l.conn.LocalAddr()
}

// Run reads messages until the context is cancelled.
func (l *Link) Run(ctx context.Context, handlers Handlers) {
	go func() {
		<-ctx.Done()
		_ = l.conn.Close()
	}()
	buf := make([]byte, maxDatagram)
	for {
		n, from, err := l.conn.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				log.Info().Msg("stopping mission link")
				return
			}
			log.Warn().Err(err).Msg("mission link read failed")
			continue
		}
		l.handle(buf[:n], from, handlers)
	}
}

func (l *Link) handle(b []byte, from *net.UDPAddr, handlers Handlers) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		log.Warn().Err(err).Msg("ignoring unreadable mission link message")
		return
	}
	switch envelope.Type {
	case TypePackages:
		var packages Packages
		if err := json.Unmarshal(b, &packages); err != nil {
			log.Warn().Err(err).Msg("ignoring unreadable packages message")
			return
		}
		l.setPeer(TypePackages, from)
		log.Info().Int("packages", len(packages.Packages)).Strs("controllers", packages.Controllers).Msg("received packages from mission")
		if handlers.OnPackages != nil {
			handlers.OnPackages(packages)
		}
	case TypeSay:
		var say Say
		if err := json.Unmarshal(b, &say); err != nil {
			log.Warn().Err(err).Msg("ignoring unreadable say message")
			return
		}
		if handlers.OnSay != nil && say.Text != "" {
			handlers.OnSay(say)
		}
	case TypeTune:
		var tune Tune
		if err := json.Unmarshal(b, &tune); err != nil {
			log.Warn().Err(err).Msg("ignoring unreadable tune message")
			return
		}
		if handlers.OnTune != nil && len(tune.Frequencies) > 0 {
			handlers.OnTune(tune)
		}
	case TypeEmitter:
		var emitter Emitter
		if err := json.Unmarshal(b, &emitter); err != nil {
			log.Warn().Err(err).Msg("ignoring unreadable emitter message")
			return
		}
		if handlers.OnEmitter != nil && emitter.Label != "" && len(emitter.Frequencies) > 0 {
			handlers.OnEmitter(emitter)
		}
	case TypeATC:
		var atc ATC
		if err := json.Unmarshal(b, &atc); err != nil {
			log.Warn().Err(err).Msg("ignoring unreadable atc message")
			return
		}
		l.setPeer(TypeATC, from)
		if handlers.OnATC != nil {
			handlers.OnATC(atc)
		}
	default:
		log.Warn().Str("type", envelope.Type).Msg("ignoring unknown mission link message")
	}
}

func (l *Link) setPeer(owner string, from *net.UDPAddr) {
	l.peerLock.Lock()
	defer l.peerLock.Unlock()
	l.peers[owner] = from
}

func (l *Link) sendTo(owner string, message any) error {
	l.peerLock.RLock()
	peer := l.peers[owner]
	l.peerLock.RUnlock()
	if peer == nil {
		return fmt.Errorf("no mission has sent a %s message to the mission link", owner)
	}
	b, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}
	if _, err := l.conn.WriteToUDP(b, peer); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	return nil
}

// Send sends a package request to the mission. It fails if no packages message has arrived yet.
func (l *Link) Send(request Request) error {
	request.Type = TypeRequest
	return l.sendTo(TypePackages, request)
}

// SendATC sends an ATC request to the mission. It fails if no atc message has arrived yet.
func (l *Link) SendATC(request ATCRequest) error {
	request.Type = TypeATCRequest
	return l.sendTo(TypeATC, request)
}
