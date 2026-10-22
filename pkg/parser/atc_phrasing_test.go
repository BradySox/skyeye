package parser

import (
	"testing"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/stretchr/testify/require"
)

type atcPhrasingCase struct {
	text     string
	callsign string
	kind     brevity.ATCRequestKind
	numbers  []string
}

func checkATCPhrasing(t *testing.T, loose bool, cases []atcPhrasingCase) {
	t.Helper()
	for _, test := range cases {
		t.Run(test.text, func(t *testing.T) {
			t.Parallel()
			actual := ParseATC(test.text, loose)
			if test.kind == "" {
				require.Nil(t, actual)
				return
			}
			require.NotNil(t, actual)
			require.Equal(t, test.callsign, actual.Callsign)
			require.Equal(t, test.kind, actual.Kind)
			if test.numbers != nil {
				require.Equal(t, test.numbers, actual.Numbers)
			}
		})
	}
}

// Strict: the calls pilots make to Ground and Tower, including the 2026-10-21 transcripts.
func TestParseATCNaturalPhrasing(t *testing.T) {
	t.Parallel()
	checkATCPhrasing(t, false, []atcPhrasingCase{
		// Readbacks with the callsign first, last, or after the station.
		{"Ford 9, cleared for takeoff, runway 28L.", "ford 9", brevity.ATCReadback, []string{"28"}},
		{"Ford 9, cleared for takeoff.", "ford 9", brevity.ATCReadback, nil},
		{"Ford 9, cleared for takeoff runway 28L.", "ford 9", brevity.ATCReadback, []string{"28"}},
		{"Ford 9, squawk 4101, taxi runway 28L, hold short.", "ford 9", brevity.ATCReadback, []string{"4101", "28"}},
		{"Ford 9, squawk 4101, hold short 28.", "ford 9", brevity.ATCReadback, []string{"4101", "28"}},
		{"Ford 9, cleared to land runway 28 left.", "ford 9", brevity.ATCReadback, []string{"28"}},
		{"Ford 9-1, roger, squawk 4101, hold short 28 left.", "ford 9 1", brevity.ATCReadback, []string{"4101", "28"}},
		{"Tower, Ford 9, cleared for takeoff 28L.", "ford 9", brevity.ATCReadback, []string{"28"}},
		{"Hatzerim Tower, Ford 9-1, cleared to land 28L.", "ford 9 1", brevity.ATCReadback, []string{"28"}},
		{"Squawk 4101, hold short 28L, Ford 9.", "ford 9", brevity.ATCReadback, []string{"4101", "28"}},
		{"Cleared for takeoff 28L, Ford 9.", "ford 9", brevity.ATCReadback, []string{"28"}},
		{"Runway 28L, cleared for takeoff, Ford 9.", "ford 9", brevity.ATCReadback, []string{"28"}},
		{"Cleared to land, Ford 9.", "ford 9", brevity.ATCReadback, nil},
		// A clearance word is never a callsign.
		{"Hold short 28.", "", "", nil},
		{"Cleared for takeoff runway 28.", "", "", nil},
		// A request without the station stays unanswered in Strict.
		{"Ford 9, request taxi.", "", "", nil},
		{"Ford 9, holding short 28L, ready.", "", "", nil},
		// Flight size and distances are not ship numbers.
		{"Hatzerim Ground, Ford 9, two ship, ready to taxi.", "ford 9", brevity.ATCTaxi, nil},
		{"Hatzerim Tower, Ford 9, two ship holding short 28L.", "ford 9", brevity.ATCReady, nil},
		{"Hatzerim Tower, Ford 9, inbound, two ship, full stop.", "ford 9", brevity.ATCInbound, nil},
		{"Hatzerim Tower, Ford 9, 10 miles west, request straight in.", "ford 9", brevity.ATCInbound, []string{"10"}},
		{"Hatzerim Approach, Ford 9, 30 miles north, inbound.", "ford 9", brevity.ATCInbound, []string{"30"}},
		{"Hatzerim Tower, Ford 9-2, 5 miles, full stop.", "ford 9 2", brevity.ATCInbound, []string{"5"}},
		{"Hatzerim Tower, Ford 9-1, 12 miles out, RTB.", "ford 9 1", brevity.ATCInbound, []string{"12"}},
		{"Hatzerim Tower, Ford 9 1 12 miles out.", "ford 9 1", brevity.ATCInbound, []string{"12"}},
		{"Hatzerim Tower, Ford 9, 5 miles, full stop.", "ford 9", brevity.ATCInbound, []string{"5"}},
		// Departure words.
		{"Tower, Ford 9, outbound.", "ford 9", brevity.ATCAirborne, nil},
		{"Hazen Tower, Uzi 2, Outbound Runway 28L.", "uzi 2", brevity.ATCAirborne, nil},
		{"Tower, Ford 9, departing runway 28.", "ford 9", brevity.ATCAirborne, nil},
		{"Tower, Ford 9 wheels up, switching.", "ford 9", brevity.ATCAirborne, nil},
		{"Tower, Ford 9, request frequency change.", "ford 9", brevity.ATCAirborne, nil},
		{"Hatzerim Tower, Ford 9, airborne.", "ford 9", brevity.ATCAirborne, nil},
		// Arrival words.
		{"Tower, Ford 9, on final, 28 left.", "ford 9", brevity.ATCInbound, nil},
		{"Tower, Ford 9, three mile final, gear down.", "ford 9", brevity.ATCInbound, nil},
		{"Tower, Ford 9, request visual approach runway 28.", "ford 9", brevity.ATCInbound, nil},
		// Radio checks, say again and emergencies.
		{"Hatzerim Ground, Ford 9, comm check.", "ford 9", brevity.ATCRadioCheck, nil},
		{"Hatzerim Ground, Ford 9, comms check.", "ford 9", brevity.ATCRadioCheck, nil},
		{"Hatzerim Ground, Ford 9, mic check.", "ford 9", brevity.ATCRadioCheck, nil},
		{"Hatzerim Ground, Ford 9, how do you read?", "ford 9", brevity.ATCRadioCheck, nil},
		{"Hatzerim, Ground, Ford 9, Radio Check.", "ford 9", brevity.ATCRadioCheck, nil},
		{"Tower, Ford 9, repeat last.", "ford 9", brevity.ATCSayAgain, nil},
		{"Tower, Ford 9, say again.", "ford 9", brevity.ATCSayAgain, nil},
		{"Pan pan, Ford 9, hydraulic failure.", "ford 9", brevity.ATCEmergency, nil},
		{"Tower, Ford 9, pan pan, hydraulic failure.", "ford 9", brevity.ATCEmergency, nil},
		{"Mayday, mayday, mayday, Ford 9, emergency fuel.", "ford 9", brevity.ATCEmergency, nil}, //nolint:dupword
		// The 2026-10-21 transcripts that worked before still work.
		{"- Hatzerim, Ground, Ford 9-Flight, ready to taxi.", "ford 9", brevity.ATCTaxi, nil},
		{"Azerim Tower, Ford 9, holding short, runway 28L.", "ford 9", brevity.ATCReady, nil},
		{"Hatzerim Ground, Ford 9, request taxi.", "ford 9", brevity.ATCTaxi, nil},
		{"Hatzerim Tower, Ford 9, holding short runway 28L, ready.", "ford 9", brevity.ATCReady, []string{"28"}},
		{"Tower, Ford 9, clear of the runway.", "ford 9", brevity.ATCClear, nil},
	})
}

// Casual: a readback with the callsign first is a readback, never a new request.
func TestParseATCLooseReadbacks(t *testing.T) {
	t.Parallel()
	checkATCPhrasing(t, true, []atcPhrasingCase{
		{"Ford 9, cleared for takeoff, runway 28L.", "ford 9", brevity.ATCReadback, []string{"28"}},
		{"Ford 9, squawk 4101, taxi runway 28L, hold short.", "ford 9", brevity.ATCReadback, []string{"4101", "28"}},
		{"Ford 9, request taxi.", "ford 9", brevity.ATCTaxi, nil},
		{"Ford 9, holding short 28L, ready.", "ford 9", brevity.ATCReady, nil},
		{"Uzi 2, outbound, runway 28.", "uzi 2", brevity.ATCAirborne, nil},
	})
}

func TestATCTokensBreakNumbersWrittenApart(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{"ford", "9", ",", "1", "0", "miles"}, atcTokens("Ford 9, 10 miles"))
	require.Equal(t, []string{"viper", "1", "1"}, atcTokens("Viper 1-1"))
	require.Equal(t, []string{"3", "0", "0", "0", "golf"}, atcTokens("3,000, Golf"))
}
