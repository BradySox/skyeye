package parser

import (
	"testing"

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
	for _, text := range []string{"Chainsaw, Ford 9, picture", "Anyface, Ford 9, picture", "Ford 9, ready"} {
		assert.Nil(t, p.ParseELINT(text, "Vacuum"), text)
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
