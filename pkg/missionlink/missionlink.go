// Package missionlink is a local UDP link between SkyEye and a mission script running a package
// controller (for example, DCS Retribution's RetLab package controller).
//
// The mission sends SkyEye its packages (frequencies and controller callsigns) and lines to speak.
// SkyEye sends the mission the package requests it heard. Messages are one JSON object per UDP
// datagram. SkyEye replies to the address the last packages message came from.
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
}

// Say asks SkyEye to speak a line on a frequency.
type Say struct {
	Type      string `json:"type"`
	Frequency string `json:"frequency"`
	Text      string `json:"text"`
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
}

// Link is the UDP link.
type Link struct {
	conn     *net.UDPConn
	peer     *net.UDPAddr
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
	return &Link{conn: conn}, nil
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
		l.peerLock.Lock()
		l.peer = from
		l.peerLock.Unlock()
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
	default:
		log.Warn().Str("type", envelope.Type).Msg("ignoring unknown mission link message")
	}
}

// Send sends a request to the mission. It fails if no packages message has arrived yet.
func (l *Link) Send(request Request) error {
	l.peerLock.RLock()
	peer := l.peer
	l.peerLock.RUnlock()
	if peer == nil {
		return errors.New("no mission has connected to the mission link")
	}
	request.Type = TypeRequest
	b, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}
	if _, err := l.conn.WriteToUDP(b, peer); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	return nil
}
