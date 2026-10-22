package parser

import (
	"testing"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/stretchr/testify/require"
)

func newPhrasingParser() *Parser {
	p := New("Image", []string{}, false)
	p.SetPackageControllers([]string{"Blackjack"})
	return p
}

// The calls pilots make to the AWACS and the ABCCC, including the 2026-10-21 transcripts.
func TestParserNaturalPhrasing(t *testing.T) {
	t.Parallel()
	pkg := func(callsign string, kind brevity.PackageRequestKind) *brevity.PackageRequest {
		return &brevity.PackageRequest{Callsign: callsign, Controller: "blackjack", Kind: kind}
	}
	testCases := []struct {
		text     string
		expected any
	}{
		// Check-in aliases, and numbers after the callsign never read into it.
		{"Image, Ford 9, with you, 161 for 313.", &brevity.CheckInRequest{Callsign: "ford 9"}},
		{"Image, Ford 9-1, on station.", &brevity.CheckInRequest{Callsign: "ford 9 1"}},
		{"Image, Ford 9-1, checked in.", &brevity.CheckInRequest{Callsign: "ford 9 1"}},
		{"Image, Ford 9-1, checking in, angels 20.", &brevity.CheckInRequest{Callsign: "ford 9 1"}},
		{"Blackjack, Ford 9, check it in.", pkg("ford 9", brevity.PackageCheckIn)},
		{"Blackjack, Ford 9-1, checking in, angels 20.", pkg("ford 9 1", brevity.PackageCheckIn)},
		{"Blackjack, Ford 9-1, with you.", pkg("ford 9 1", brevity.PackageCheckIn)},
		{"Blackjack, Ford 9-1, on station.", pkg("ford 9 1", brevity.PackageCheckIn)},
		// A bare "ready" answers the roll call.
		{"Blackjack, Ford 9-1, ready.", pkg("ford 9 1", brevity.PackageCheckIn)},
		{"Blackjack, Ford 9 ready", pkg("ford 9", brevity.PackageCheckIn)},
		{"Blackjack, Ford 9-1, ready to push.", pkg("ford 9 1", brevity.PackageReadyToPush)},
		// The controller after the callsign.
		{"Ford 9, Image, say picture.", &brevity.PictureRequest{Callsign: "ford 9"}},
		{"Ford 9-1, Image, radio check.", &brevity.RadioCheckRequest{Callsign: "ford 9 1"}},
		{"Ford 9 1, Blackjack, check in.", pkg("ford 9 1", brevity.PackageCheckIn)},
		// A flight size is not a ship number.
		{"Image, Ford 9, two ship, picture.", &brevity.PictureRequest{Callsign: "ford 9"}},
		{"Image, Ford 9, four ship, checking in.", &brevity.CheckInRequest{Callsign: "ford 9"}},
		// A call the ABCCC cannot read gets its own say again; an acknowledgement gets nothing.
		{"Blackjack, Ford 9-1, say that again.", pkg("ford 9 1", brevity.PackageUnknown)},
		{"Blackjack, Ford 9-1, uh.", pkg("ford 9 1", brevity.PackageUnknown)},
		{"Blackjack, Ford 9-1, copy.", nil},
		{"Blackjack, Ford 9-1, wilco.", nil},
		// Unchanged.
		{"Image, Ford 9-Picture", &brevity.PictureRequest{Callsign: "ford 9"}},
		{"Blackjack, Ford 9-1, check in.", pkg("ford 9 1", brevity.PackageCheckIn)},
		{"Blackjack, Ford 9-1, say push time.", pkg("ford 9 1", brevity.PackagePushTime)},
	}
	p := newPhrasingParser()
	for _, test := range testCases {
		t.Run(test.text, func(t *testing.T) {
			t.Parallel()
			actual := p.Parse(test.text)
			if test.expected == nil {
				require.Nil(t, actual)
				return
			}
			require.Equal(t, test.expected, actual)
		})
	}
}

func TestParserCallsignBeforeControllerNeedsANumber(t *testing.T) {
	t.Parallel()
	require.Nil(t, newPhrasingParser().Parse("Nice shot, Image, picture."))
}

func TestParseLoosePackageCallStopsAtTheCallsign(t *testing.T) {
	t.Parallel()
	require.Equal(t,
		&brevity.PackageRequest{Callsign: "ford 9 1", Controller: "blackjack", Kind: brevity.PackageCheckIn},
		newPhrasingParser().ParseLoosePackageCall("Ford 9-1, checking in, angels 20."))
}

func TestLeadingCallsign(t *testing.T) {
	t.Parallel()
	testCases := map[string]string{
		"ford 9 1 check in 161 for 313": "ford 9 1",
		"ford 9 check in 161 for 313":   "ford 9",
		"ford 9 1 10 miles":             "ford 9 1",
		"ford 9 30 miles north":         "ford 9",
		"ford 9 2 ship":                 "ford 9",
		"ford 9 1":                      "ford 9 1",
		"ford 91":                       "ford 91",
		"eagle won won":                 "eagle won won", //nolint:dupword
		"who is saying 1 on 1":          "who is saying 1 on 1",
		"gunfighter":                    "gunfighter",
	}
	for in, expected := range testCases {
		require.Equal(t, expected, leadingCallsign(in), in)
	}
}
