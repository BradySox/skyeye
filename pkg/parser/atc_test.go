package parser

import (
	"testing"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/stretchr/testify/require"
)

func TestParseATC(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		text     string
		expected *brevity.ATCRequest
	}{
		{
			text: "Kobuleti Ground, Viper 1-1, request engine start.",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Position: positionGround, Kind: brevity.ATCStartup,
				Text: "kobuleti ground viper 1 1 request engine start",
			},
		},
		{
			text: "Cobb Lady Ground, this is Viper 11, request taxi",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Position: positionGround, Kind: brevity.ATCTaxi,
				Text: "cobb lady ground this is viper 1 1 request taxi",
			},
		},
		{
			text: "Kobuleti Tower, Viper 1-1, holding short runway 25, ready.",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Position: positionTower, Kind: brevity.ATCReady, Numbers: []string{"25"},
				Text: "kobuleti tower viper 1 1 holding short runway 2 5 ready",
			},
		},
		{
			text: "Tower, Viper 1-1, 5 miles, request the overhead.",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Position: positionTower, Kind: brevity.ATCInbound, Detail: "overhead", Numbers: []string{"5"},
				Text: "tower viper 1 1 5 miles request the overhead",
			},
		},
		{
			text: "Kobuleti Tower, Viper won won, five miles, full stop",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Position: positionTower, Kind: brevity.ATCInbound, Numbers: []string{"5"},
				Text: "kobuleti tower viper won won 5 miles full stop", //nolint:dupword
			},
		},
		{
			text: "Kobuleti Tower, Viper 1-1, clear of the runway.",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Position: positionTower, Kind: brevity.ATCClear,
				Text: "kobuleti tower viper 1 1 clear of the runway",
			},
		},
		{
			text: "Kobuleti Departure, Viper 1-1, airborne",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Position: positionDeparture, Kind: brevity.ATCAirborne,
				Text: "kobuleti departure viper 1 1 airborne",
			},
		},
		{
			text: "Ground, Viper 1-1, radio check",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Position: positionGround, Kind: brevity.ATCRadioCheck,
				Text: "ground viper 1 1 radio check",
			},
		},
		{
			text: "Mayday, mayday, Viper 1-1, emergency fuel",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Kind: brevity.ATCEmergency,
				Text: "mayday mayday viper 1 1 emergency fuel", //nolint:dupword
			},
		},
		{
			text: "Squawk 4101, Alpha, hold short 25, Viper 1-1.",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Kind: brevity.ATCReadback, Numbers: []string{"4101", "25"},
				Text: "squawk 4 1 0 1 alpha hold short 2 5 viper 1 1",
			},
		},
		{
			text: "Squawk four one zero one, hold short two five, contact tower two seven zero decimal four five, Viper one one", //nolint:dupword
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Kind: brevity.ATCReadback, Numbers: []string{"4101", "25", "270.45"},
				Text: "squawk 4 1 0 1 hold short 2 5 contact tower 2 7 0 . 4 5 viper 1 1",
			},
		},
		{
			text: "3,000, Golf, Tower at 5, Viper 1-1",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Kind: brevity.ATCReadback, Numbers: []string{"3000", "5"},
				Text: "3 0 0 0 golf tower at 5 viper 1 1",
			},
		},
		{
			text: "Three thousand, Golf, Viper 1-1",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Kind: brevity.ATCReadback, Numbers: []string{"3000"},
				Text: "3 thousand golf viper 1 1",
			},
		},
		{
			text: "Cleared for takeoff 25, Viper 1-1",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Kind: brevity.ATCReadback, Numbers: []string{"25"},
				Text: "cleared for takeoff 2 5 viper 1 1",
			},
		},
		{
			text: "Kobuleti Tower, Viper 1-1, uh",
			expected: &brevity.ATCRequest{
				Callsign: "viper 1 1", Position: positionTower, Kind: brevity.ATCUnknown,
				Text: "kobuleti tower viper 1 1 uh",
			},
		},
		{text: "Two, three.", expected: nil},
		{text: "", expected: nil},
		{text: "Kobuleti Tower, request taxi", expected: nil},
	}
	for _, test := range testCases {
		t.Run(test.text, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.expected, ParseATC(test.text))
		})
	}
}
