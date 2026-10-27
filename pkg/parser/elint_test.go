package parser

import (
	"testing"
	"time"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newELINTParser() *Parser {
	p := New("Chainsaw", nil, false)
	p.SetSpeaker("Vacuum")
	return p
}

func TestParseELINT(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		text     string
		kind     brevity.ELINTRequestKind
		callsign string
		bearing  string
	}{
		{"Vacuum, Ford 9, checking in.", brevity.ELINTCheckIn, "ford 9", ""},
		{"Vacuum, Ford 9, check in", brevity.ELINTCheckIn, "ford 9", ""},
		{"Vacuum, Ford 9, Ford 9, checking in", brevity.ELINTCheckIn, "ford 9", ""},
		{"Vacuum, Ford 9, emitters", brevity.ELINTEmitters, "ford 9", ""},
		{"Vacuum, Ford 9, say emitters", brevity.ELINTEmitters, "ford 9", ""},
		{"Vacuum, Ford 9, request emitters", brevity.ELINTEmitters, "ford 9", ""},
		{"Vacuum, Ford 9, uh, give me emitters", brevity.ELINTEmitters, "ford 9", ""},
		{"Vacuum, Ford 9, what's the nearest emitter?", brevity.ELINTEmitters, "ford 9", ""},
		{"Vacuum, Ford 9, send emitters", brevity.ELINTEmitters, "ford 9", ""},
		{"Vacuum, Ford 9, nearest", brevity.ELINTEmitters, "ford 9", ""},
		{"Vacuum, Ford 9, a meter", brevity.ELINTEmitters, "ford 9", ""},
		{"Vacuum, Ford 9, status target", brevity.ELINTTarget, "ford 9", ""},
		{"Vacuum, Ford 9, request target status", brevity.ELINTTarget, "ford 9", ""},
		{"Vacuum, Ford 9, say status target", brevity.ELINTTarget, "ford 9", ""},
		{"Vacuum, Ford 9, how about the target?", brevity.ELINTTarget, "ford 9", ""},
		{"Vacuum, Ford 9, mud 270", brevity.ELINTMud, "ford 9", "270"},
		{"Vacuum, Ford 9, mud two seven zero", brevity.ELINTMud, "ford 9", "270"},
		{"Vacuum, Ford 9, mud at 270", brevity.ELINTMud, "ford 9", "270"},
		{"Vacuum, Ford 9, mud spike 0 9 0", brevity.ELINTMud, "ford 9", "090"},
		{"Vacuum, Ford 9, dirt 45", brevity.ELINTMud, "ford 9", "045"},
		{"Vacuum, Ford 9, mod 270", brevity.ELINTMud, "ford 9", "270"},
		{"Vacuum, Ford 9, mud", brevity.ELINTMud, "ford 9", ""},
		{"Vacuum, Ford 9, say again", brevity.ELINTSayAgain, "ford 9", ""},
		{"Vacuum, Ford 9, say again last", brevity.ELINTSayAgain, "ford 9", ""},
		{"Vacuum, Ford 9, say again your last", brevity.ELINTSayAgain, "ford 9", ""},
		{"Vacuum, Viper 1-1, say emitters", brevity.ELINTEmitters, "viper 1 1", ""},
		{"Vacuum, Ford 9, picture", brevity.ELINTUnknown, "ford 9", ""},
		// The way pilots talk (natural phrasing): Vacuum after the callsign, check-in aliases,
		// flight sizes and numbers after the callsign.
		{"Ford 9, Vacuum, say emitters", brevity.ELINTEmitters, "ford 9", ""},
		{"Ford 9 1, Vacuum, request target status", brevity.ELINTTarget, "ford 9 1", ""},
		{"Ford 9, Vacuum, checking in", brevity.ELINTCheckIn, "ford 9", ""},
		{"Vacuum, Ford 9, with you", brevity.ELINTCheckIn, "ford 9", ""},
		{"Vacuum, Ford 9, on station", brevity.ELINTCheckIn, "ford 9", ""},
		{"Vacuum, Ford 9, checked in", brevity.ELINTCheckIn, "ford 9", ""},
		{"Vacuum, Ford 9, two ship, checking in", brevity.ELINTCheckIn, "ford 9", ""},
		{"Vacuum, Ford 9-1, mud two seven zero", brevity.ELINTMud, "ford 9 1", "270"},
		{"Ford 9-1, Vacuum, mud 270", brevity.ELINTMud, "ford 9 1", "270"},
		// Whisper glues the bearing onto a radar name (2026-10-26): the SA number is not the bearing.
		{"Vacuum, Enfield 4-1, MUD, SA-6-185.", brevity.ELINTMud, "enfield 4 1", "185"},
		{"Vacuum, Ford 9, mud SA-6 185", brevity.ELINTMud, "ford 9", "185"},
		{"Vacuum, Ford 9, mud, SA 6, 1 8 5", brevity.ELINTMud, "ford 9", "185"},
		{"Vacuum, Ford 9, mud S-A-6 185", brevity.ELINTMud, "ford 9", "185"},
		{"Vacuum, Ford 9, mud SA6 185", brevity.ELINTMud, "ford 9", "185"},
		{"Vacuum, Ford 9, mud SA-10, one eight five", brevity.ELINTMud, "ford 9", "185"},
		{"Vacuum, Ford 9, mud SA-11-270", brevity.ELINTMud, "ford 9", "270"},
		{"Vacuum, Ford 9, mud 185 SA-6", brevity.ELINTMud, "ford 9", "185"},
		{"Vacuum, Ford 9, mud Hawk 185", brevity.ELINTMud, "ford 9", "185"},
		{"Vacuum, Ford 9, mud spike SA-6 at 185", brevity.ELINTMud, "ford 9", "185"},
		{"Vacuum, Ford 9, mod SA-6 185", brevity.ELINTMud, "ford 9", "185"},
		{"Vacuum, Ford 9, mud SA-6", brevity.ELINTMud, "ford 9", ""},
	}
	for _, test := range testCases {
		t.Run(test.text, func(t *testing.T) {
			t.Parallel()
			request := newELINTParser().ParseELINT(test.text, "")
			require.NotNil(t, request)
			assert.Equal(t, test.kind, request.Kind)
			assert.Equal(t, test.callsign, request.Callsign)
			assert.Equal(t, test.bearing, request.Bearing)
			assert.Equal(t, "vacuum", request.Speaker)
		})
	}
}

func TestParseELINTOnOwnFrequency(t *testing.T) {
	t.Parallel()
	p := newELINTParser()
	for _, text := range []string{"Ford 9, checking in", "Anyface, Ford 9, say emitters", "Ford 9, mud 270"} {
		request := p.ParseELINT(text, "Vacuum")
		require.NotNil(t, request, text)
		assert.Equal(t, "ford 9", request.Callsign, text)
		assert.Equal(t, "vacuum", request.Speaker, text)
	}
	// Calls that are not for Vacuum stay with the GCI.
	for _, text := range []string{"Chainsaw, Ford 9, picture", "Anyface, Ford 9, picture", "Ford 9, ready", "Ford 9, Chainsaw, picture", "Ford 9, Chainsaw, with you"} {
		assert.Nil(t, p.ParseELINT(text, "Vacuum"), text)
	}
	// A check-in by an alias on Vacuum's frequency, naming no one, is Vacuum's.
	request := p.ParseELINT("Ford 9, with you", "Vacuum")
	require.NotNil(t, request)
	assert.Equal(t, brevity.ELINTCheckIn, request.Kind)
	assert.Equal(t, "ford 9", request.Callsign)
}

// A check-in alias addressed to Vacuum is Vacuum's, never the GCI's; one to the GCI stays the GCI's.
func TestParseELINTCheckInAliasesGoToTheirAddressee(t *testing.T) {
	t.Parallel()
	p := newELINTParser()
	request := p.ParseELINT("Vacuum, Ford 9, with you", "")
	require.NotNil(t, request)
	assert.Equal(t, brevity.ELINTCheckIn, request.Kind)
	for _, text := range []string{"Chainsaw, Ford 9, with you", "Ford 9, Chainsaw, with you"} {
		assert.Nil(t, p.ParseELINT(text, ""), text)
		_, ok := p.Parse(text).(*brevity.CheckInRequest)
		assert.True(t, ok, text)
	}
}

func TestParseELINTIgnoresOtherCalls(t *testing.T) {
	t.Parallel()
	p := newELINTParser()
	for _, text := range []string{"Chainsaw, Ford 9, picture", "Anyface, Ford 9, say emitters", "Ford 9, mud 270", "Chainsaw, Ford 9, strobe 270"} {
		assert.Nil(t, p.ParseELINT(text, ""), text)
	}
	assert.Nil(t, New("Chainsaw", nil, false).ParseELINT("Vacuum, Ford 9, emitters", ""))
	// The GCI still parses its own calls.
	_, ok := p.Parse("Chainsaw, Ford 9, picture").(*brevity.PictureRequest)
	assert.True(t, ok)
}

// After "say mud bearing" the same caller's bare bearing is that mud call, for 30 seconds.
func TestParseMudFollowUp(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 26, 20, 0, 0, 0, time.UTC)
	mud := func(p *Parser) {
		request := p.ParseELINT("Vacuum, Enfield 4-1, mud", "")
		require.NotNil(t, request)
		require.Empty(t, request.Bearing)
		p.AwaitMudBearing("Enfield|262.0", request, start)
	}
	for _, text := range []string{"1-8-0", "one eight zero", "180", "mud 180", "uh, 1 8 0."} {
		p := newELINTParser()
		mud(p)
		request := p.ParseMudFollowUp(text, "Enfield|262.0", start.Add(10*time.Second))
		require.NotNil(t, request, text)
		assert.Equal(t, brevity.ELINTMud, request.Kind, text)
		assert.Equal(t, "180", request.Bearing, text)
		assert.Equal(t, "enfield 4 1", request.Callsign, text)
		assert.Equal(t, "vacuum", request.Speaker, text)
		// Answered once only.
		assert.Nil(t, p.ParseMudFollowUp(text, "Enfield|262.0", start.Add(11*time.Second)), text)
	}

	p := newELINTParser()
	mud(p)
	assert.Nil(t, p.ParseMudFollowUp("180", "Ford|262.0", start.Add(time.Second)), "another caller")
	assert.Nil(t, p.ParseMudFollowUp("Ford 9, checking in", "Enfield|262.0", start.Add(time.Second)))
	assert.Nil(t, p.ParseMudFollowUp("180", "Enfield|262.0", start.Add(31*time.Second)), "too late")

	// A mud call with its bearing waits for nothing.
	p = newELINTParser()
	p.AwaitMudBearing("Enfield|262.0", p.ParseELINT("Vacuum, Enfield 4-1, mud 270", ""), start)
	assert.Nil(t, p.ParseMudFollowUp("180", "Enfield|262.0", start.Add(time.Second)))
	assert.Nil(t, newELINTParser().ParseMudFollowUp("180", "", start))
}
