package controller

import (
	"testing"

	"github.com/dharmab/skyeye/pkg/coalitions"
	"github.com/paulmach/orb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A threat or merged call names a player by the flight callsign the mission says they fly, as
// bogey dope answers them, not by their in-game name.
func TestFriendlyCallsignUsesMissionPilots(t *testing.T) {
	t.Parallel()
	h := newControllerTestHarness(t, nil)
	h.insertAircraft(t, "Flash", acmiF16C, coalitions.Blue, orb.Point{30.1, 40.1})
	h.insertAircraft(t, "Blade", acmiF16C, coalitions.Blue, orb.Point{30.2, 40.1})
	h.insertAircraft(t, "Eagle 1 Reaper", acmiF15C, coalitions.Blue, orb.Point{30.3, 40.1})
	flash := h.rdr.FindUnit(1)
	require.NotNil(t, flash)
	reaper := h.rdr.FindUnit(3)
	require.NotNil(t, reaper)

	callsign, ok := h.ctrl.friendlyCallsign(flash)
	require.True(t, ok)
	assert.Equal(t, "flash", callsign)

	h.rdr.SetCallsignAliases(map[string]string{"Ford 9-1": "Flash", "Ford 9": "Flash", "Ford 9-2": "Blade"})
	callsign, ok = h.ctrl.friendlyCallsign(flash)
	require.True(t, ok)
	assert.Equal(t, "ford 9 1", callsign)
	callsign, ok = h.ctrl.friendlyCallsign(reaper)
	require.True(t, ok)
	assert.Equal(t, "eagle 1", callsign)

	// Both seats of the flight are threatened, so the call collates them as the flight.
	everyone := h.ctrl.getFriendlyCallsigns()
	assert.ElementsMatch(t, []string{"ford 9 1", "ford 9 2", "eagle 1"}, everyone)
	assert.Equal(t, []string{"ford 9 flight"}, collateCallsigns([]string{"ford 9 1", "ford 9 2"}, everyone))
}
