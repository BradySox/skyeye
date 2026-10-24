package parser

import (
	"testing"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParserPicture(t *testing.T) {
	t.Parallel()
	testCases := []parserTestCase{
		{
			text: "anyface, intruder 1-1 request picture",
			expected: &brevity.PictureRequest{
				Callsign: "intruder 1 1",
			},
		},
		{
			text: "anyface, intruder 1-1 picture 30",
			expected: &brevity.PictureRequest{
				Callsign: "intruder 1 1",
			},
		},
		{
			text: "anyface, picture",
			expected: &brevity.PictureRequest{
				Callsign: "",
			},
		},
		{
			text: "anyface, Ford 1-1, picture",
			expected: &brevity.PictureRequest{
				Callsign: "ford 1 1",
			},
		},
	}
	runParserTestCases(t, New(TestCallsign, []string{}, true), testCases, func(t *testing.T, test parserTestCase, request any) {
		t.Helper()
		expected := test.expected.(*brevity.PictureRequest)
		actual := request.(*brevity.PictureRequest)
		assert.Equal(t, expected.Callsign, actual.Callsign)
	})
}

func TestParserPictureLeadInsAreNotACallsign(t *testing.T) {
	t.Parallel()
	p := New("Chainsaw", []string{}, true)
	testCases := []struct {
		text     string
		callsign string
	}{
		{"- Chainsaw, say picture.", ""},
		{"Chainsaw, request picture", ""},
		{"Chainsaw, uh, give me the picture", ""},
		{"Chainsaw, what's the picture", ""},
		{"Chainsaw, Chevy 2, say picture.", "chevy 2"},
		{"Chainsaw, Chevy 2 1, requesting picture", "chevy 2 1"},
		{"Chainsaw, this is Chevy 2 1, say picture", "chevy 2 1"},
		{"Chevy, Chainsaw, safe picture.", "chevy"},
		{"Chevy, Chainsaw, say picture.", "chevy"},
		{"Chainsaw, Chevy 2, safe picture.", "chevy 2"},
	}
	for _, test := range testCases {
		t.Run(test.text, func(t *testing.T) {
			t.Parallel()
			request := p.Parse(test.text)
			require.IsType(t, &brevity.PictureRequest{}, request)
			assert.Equal(t, test.callsign, request.(*brevity.PictureRequest).Callsign)
		})
	}
}

func TestParserFlightNameNeedsABareRequest(t *testing.T) {
	t.Parallel()
	p := New("Chainsaw", []string{}, true)
	require.Nil(t, p.Parse("Chevy, Chainsaw, how are you doing"))
	require.Nil(t, p.Parse("Chevy, Chainsaw, it looks safe"))
}
