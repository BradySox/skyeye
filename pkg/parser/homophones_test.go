package parser

import (
	"testing"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/stretchr/testify/require"
)

// The calls are Whisper's transcripts of RetLab calls spoken on radio-quality audio (2026-10-29).

func newSweepParser(gci string) *Parser {
	p := New(gci, []string{}, false)
	p.SetPackageControllers([]string{"Ramrod", "Anvil", "Blackjack"})
	p.SetSpeaker("Vacuum")
	return p
}

func TestHeardRomanNumeralCallsign(t *testing.T) {
	t.Parallel()
	request := newSweepParser("Chainsaw").Parse("Chainsaw, Chevy II, Bogeydope.")
	require.IsType(t, &brevity.BogeyDopeRequest{}, request)
	require.Equal(t, "chevy 2", request.(*brevity.BogeyDopeRequest).Callsign)

	atc := ParseATC("Kandahar Tower, Springfield II, ready for takeoff.", true)
	require.NotNil(t, atc)
	require.Equal(t, "springfield 2", atc.Callsign)
}

func TestHeardFordForFour(t *testing.T) {
	t.Parallel()
	p := newSweepParser("Chainsaw")
	require.Equal(t,
		&brevity.PackageRequest{Callsign: "dodge 4", Controller: "anvil", Kind: brevity.PackageCheckIn},
		p.Parse("Anvil, Dodge, Ford, check in."))
	// Ford stays a name after a controller.
	request := p.Parse("Chainsaw, Ford 9-1, bogey dope.")
	require.IsType(t, &brevity.BogeyDopeRequest{}, request)
	require.Equal(t, "ford 9 1", request.(*brevity.BogeyDopeRequest).Callsign)
	request = newSweepParser("Image").Parse("Image, Ford, bogey dope.")
	require.IsType(t, &brevity.BogeyDopeRequest{}, request)
	require.Equal(t, "ford", request.(*brevity.BogeyDopeRequest).Callsign)
}

func TestHeardAnvil(t *testing.T) {
	t.Parallel()
	p := newSweepParser("Chainsaw")
	require.Equal(t,
		&brevity.PackageRequest{Callsign: "colt 1 1", Controller: "anvil", Kind: brevity.PackageOffTarget},
		p.Parse("Amble, Colt 1-1, off target."))
	require.Equal(t,
		&brevity.PackageRequest{Callsign: "ford 9 1", Controller: "anvil", Kind: brevity.PackageAirborne},
		p.Parse("Envil, Ford 9-1, airborne, proceeding to hold."))
}

func TestHeardGCIRequestWords(t *testing.T) {
	t.Parallel()
	p := newSweepParser("Chainsaw")
	for _, text := range []string{
		"Chainsaw, Dodge 4, declare, BULSAI 180-25, Angels 20.",
		"Chainsaw, Dodge 4, De'Flare, Bullseye 180, 25, Angels 20.",
	} {
		request := p.Parse(text)
		require.IsType(t, &brevity.DeclareRequest{}, request, text)
		require.False(t, request.(*brevity.DeclareRequest).IsAmbiguous, text)
	}
	require.Equal(t, &brevity.VectorRequest{Callsign: "enfield 4 1", Location: "tanker"},
		p.Parse("Chainsaw, Enfield 4-1, Becker to Tanker."))
	request := newSweepParser("Image").Parse("Image, Enfield 4-1, Bodiedope.")
	require.IsType(t, &brevity.BogeyDopeRequest{}, request)
}

func TestHeardVacuumWords(t *testing.T) {
	t.Parallel()
	p := newSweepParser("Chainsaw")
	for _, test := range []struct {
		text    string
		kind    brevity.ELINTRequestKind
		bearing string
	}{
		{"Vacuum, Uzi 21, say Mrs.", brevity.ELINTEmitters, ""},
		{"Vacuum, Colt 11, same it as.", brevity.ELINTEmitters, ""},
		{"Vacuum, Ford 9-1, SAM in misses.", brevity.ELINTEmitters, ""},
		{"Vacuum, Enfield 4-1, MUD SAR 6-180.", brevity.ELINTMud, "180"},
		{"Vacuum, Uzi 21, MUD, SA-6180.", brevity.ELINTMud, "180"},
	} {
		request := p.ParseELINT(test.text, "")
		require.NotNil(t, request, test.text)
		require.Equal(t, test.kind, request.Kind, test.text)
		require.Equal(t, test.bearing, request.Bearing, test.text)
	}
}
