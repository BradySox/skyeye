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

type atcDetailCase struct {
	text     string
	callsign string
	kind     brevity.ATCRequestKind
	detail   string
}

// The military pattern calls (2026-10-26): taxi with the numbers or a flight count, the overhead
// and straight-in, and the landing call at base or final with the gear down.
func TestParseATCPatternCalls(t *testing.T) {
	t.Parallel()
	for _, test := range []atcDetailCase{
		// Taxi and takeoff: a flight count is never a ship number.
		{"Hatzerim Ground, Viper 1-1, flight of two, request taxi.", "viper 1 1", brevity.ATCTaxi, ""},
		{"Hatzerim Ground, Viper 1, flight of four, request taxi.", "viper 1", brevity.ATCTaxi, ""},
		{"Hatzerim Ground, Viper 1-1, flight of two, have the numbers, request taxi.", "viper 1 1", brevity.ATCTaxi, DetailNumbers},
		{"Ground, Viper 1-1, request taxi with the numbers.", "viper 1 1", brevity.ATCTaxi, DetailNumbers},
		{"Tower, Viper 1-1 flight, holding short, ready for takeoff.", "viper 1 1", brevity.ATCReady, ""},
		{"Tower, Viper 1-1, have the numbers, ready for departure.", "viper 1 1", brevity.ATCReady, DetailNumbers},
		// Airborne is never a takeoff request.
		{"Tower, Viper 1-1, airborne, switching departure.", "viper 1 1", brevity.ATCAirborne, ""},
		{"Hatzerim Tower, Viper 1-1, airborne, changing to departure.", "viper 1 1", brevity.ATCAirborne, ""},
		{"Tower, Viper 1-1, ready for departure.", "viper 1 1", brevity.ATCReady, ""},
		// Inbound: overhead by default, straight-in on request.
		{"Hatzerim Tower, Viper 1-1, 10 miles, inbound, full stop.", "viper 1 1", brevity.ATCInbound, ""},
		{"Hatzerim Tower, Viper 1-1, 10 miles, request the overhead.", "viper 1 1", brevity.ATCInbound, DetailOverhead},
		{"Tower, Viper 1-1, 10 miles, request the break.", "viper 1 1", brevity.ATCInbound, DetailOverhead},
		{"Hatzerim Tower, Viper 1-1, 10 miles, request straight-in, full stop.", "viper 1 1", brevity.ATCInbound, DetailStraightIn},
		{"Tower, Viper 1-1, 15 miles, straight in.", "viper 1 1", brevity.ATCInbound, DetailStraightIn},
		{"Hatzerim Tower, Viper 1-1, 20 miles, returning to base, full stop.", "viper 1 1", brevity.ATCInbound, ""},
		// Pattern reports, with or without the station.
		{"Tower, Viper 1-1, initial.", "viper 1 1", brevity.ATCInbound, DetailInitial},
		{"Viper 1-1, initial.", "viper 1 1", brevity.ATCInbound, DetailInitial},
		{"Viper 1-1, initial, full stop.", "viper 1 1", brevity.ATCInbound, DetailInitial},
		{"Viper 1-1, break.", "viper 1 1", brevity.ATCInbound, DetailBreak},
		{"Viper 1-1, brake.", "viper 1 1", brevity.ATCInbound, DetailBreak},
		{"Viper 1-1, base, gear down, full stop.", "viper 1 1", brevity.ATCInbound, DetailGearDown},
		{"Viper 1-2, base, gear's down.", "viper 1 2", brevity.ATCInbound, DetailGearDown},
		{"Viper 1-1, 3 miles, gear down.", "viper 1 1", brevity.ATCInbound, DetailGearDown},
		{"Viper 1-1, 3 mile final, gear down.", "viper 1 1", brevity.ATCInbound, DetailGearDown},
		{"Viper 1-1, on final, gear down.", "viper 1 1", brevity.ATCInbound, DetailGearDown},
		{"Viper 1-1, gear down.", "viper 1 1", brevity.ATCInbound, DetailGearDown},
		{"Viper 1-1 flight, base, gear down.", "viper 1 1", brevity.ATCInbound, DetailGearDown},
		{"Base, gear down, Viper 1-1.", "viper 1 1", brevity.ATCInbound, DetailGearDown},
		{"Tower, Viper 1-1, 3 mile final, gear down.", "viper 1 1", brevity.ATCInbound, DetailGearDown},
		{"Viper 1-1, 3 mile final.", "viper 1 1", brevity.ATCInbound, DetailGearDown},
		// Emergencies without "mayday".
		{"Tower, Viper 1-1, engine failure.", "viper 1 1", brevity.ATCEmergency, ""},
		{"Tower, Viper 1-1, engine out, 10 miles.", "viper 1 1", brevity.ATCEmergency, ""},
		{"Tower, Viper 1-1, flameout.", "viper 1 1", brevity.ATCEmergency, ""},
		{"Tower, Viper 1-1, flame out.", "viper 1 1", brevity.ATCEmergency, ""},
		{"Tower, Viper 1-1, engine fire.", "viper 1 1", brevity.ATCEmergency, ""},
		{"Ground, Viper 1-1, request engine start.", "viper 1 1", brevity.ATCStartup, ""},
		// The runway in use (Test 65).
		{"Hatzerim Tower, Enfield 4-1, request runway in use.", "enfield 4 1", brevity.ATCRunwayInUse, ""},
		{"Hatzerim Ground, Enfield 4-1, request active runway.", "enfield 4 1", brevity.ATCRunwayInUse, ""},
		{"Tower, Enfield 4-1, say active.", "enfield 4 1", brevity.ATCRunwayInUse, ""},
		// "None" heard for "nine" after the callsign's name.
		{"Tower, Dodge none, holding short, runway 23.", "dodge 9", brevity.ATCReady, ""},
		// Hot brakes is not the break.
		{"Tower, Viper 1-1, hot brakes.", "viper 1 1", brevity.ATCUnknown, ""},
	} {
		t.Run(test.text, func(t *testing.T) {
			t.Parallel()
			actual := ParseATC(test.text, false)
			require.NotNil(t, actual)
			require.Equal(t, test.callsign, actual.Callsign)
			require.Equal(t, test.kind, actual.Kind)
			require.Equal(t, test.detail, actual.Detail)
		})
	}
}

// A pattern report without the station is a request, never a readback; a readback stays one.
func TestParseATCPatternReportsAreNotReadbacks(t *testing.T) {
	t.Parallel()
	require.Nil(t, ParseATC("Viper 1-1, flight of two.", false))
	actual := ParseATC("Viper 1-1, cleared to land runway 25.", false)
	require.NotNil(t, actual)
	require.Equal(t, brevity.ATCReadback, actual.Kind)
	actual = ParseATC("Runway 25, cleared to land, Viper 1-1.", false)
	require.NotNil(t, actual)
	require.Equal(t, brevity.ATCReadback, actual.Kind)
	actual = ParseATC("Viper 1-1, flight of two, 3 miles, gear down.", false)
	require.NotNil(t, actual)
	require.Equal(t, "viper 1 1", actual.Callsign)
	require.Equal(t, []string{"2", "3"}, actual.Numbers)
}

// "Clear" for "cleared" in a readback, in either order (Test 65); "clear of the runway" stays a
// request.
func TestParseATCClearReadbacks(t *testing.T) {
	t.Parallel()
	checkATCPhrasing(t, false, []atcPhrasingCase{
		{"Enfield 4, clear for takeoff 2-3.", "enfield 4", brevity.ATCReadback, []string{"23"}},
		{"Enfield 4-1, clear to land runway 23.", "enfield 4 1", brevity.ATCReadback, []string{"23"}},
		{"Enfield 4, runway 23, takeoff clear.", "enfield 4", brevity.ATCReadback, []string{"23"}},
		{"Clear for takeoff 23, Enfield 4.", "enfield 4", brevity.ATCReadback, []string{"23"}},
		{"Tower, Enfield 4, clear for takeoff 23.", "enfield 4", brevity.ATCReadback, []string{"23"}},
		{"Tower, Enfield 4, clear of the runway.", "enfield 4", brevity.ATCClear, nil},
	})
	checkATCPhrasing(t, true, []atcPhrasingCase{
		{"Enfield 4, clear for takeoff 2-3.", "enfield 4", brevity.ATCReadback, []string{"23"}},
	})
}

// A field name that opens a call with no station is not part of the callsign (2026-10-26 logs).
func TestTrimFieldName(t *testing.T) {
	t.Parallel()
	fields := []string{"Hatzerim", "Shindand", "Spangdahlem"}
	for _, test := range []struct {
		text     string
		callsign string
		kind     brevity.ATCRequestKind
	}{
		{"Shindand, Springfield 2, ready to taxi.", "springfield 2", brevity.ATCTaxi},
		{"Azarim, Uzi 2, outbound, runway 28.", "uzi 2", brevity.ATCAirborne},
		{"Spangdahlem, Ground, Chevy 2, ready to taxi.", "chevy 2", brevity.ATCTaxi},
		{"Springfield 2, ready to taxi.", "springfield 2", brevity.ATCTaxi},
	} {
		t.Run(test.text, func(t *testing.T) {
			t.Parallel()
			actual := ParseATC(TrimFieldName(test.text, fields), true)
			require.NotNil(t, actual)
			require.Equal(t, test.callsign, actual.Callsign)
			require.Equal(t, test.kind, actual.Kind)
		})
	}
}
