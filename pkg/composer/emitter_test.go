package composer

import (
	"strings"
	"testing"

	"github.com/dharmab/skyeye/pkg/bearings"
	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/martinlindhe/unit"
	"github.com/stretchr/testify/assert"
)

func TestComposeEmitter(t *testing.T) {
	t.Parallel()
	c := &Composer{}
	bullseye := brevity.NewBullseye(bearings.NewMagneticBearing(90*unit.Degree), 25*unit.NauticalMile)
	response := c.ComposeEmitter("Vacuum", "SA-6", "", "", bullseye)
	assert.Equal(t, "Vacuum, new emitter, SA-6, bullseye 090/25.", response.Subtitle)
	assert.Contains(t, response.Speech, "Vacuum, new emitter, S-A-6, bullseye ")

	response = c.ComposeEmitter("Vacuum", "SA-6", "", "", nil)
	assert.Equal(t, "Vacuum, new emitter, SA-6.", response.Subtitle)
}

func TestComposeEmitterAwakeAsleep(t *testing.T) {
	t.Parallel()
	c := &Composer{}
	bullseye := brevity.NewBullseye(bearings.NewMagneticBearing(304*unit.Degree), 60*unit.NauticalMile)
	response := c.ComposeEmitter("Vacuum", "SA-6", "awake", "tight", bullseye)
	assert.Equal(t, "Vacuum, SA-6 awake, bullseye 304/60, tight.", response.Subtitle)
	assert.Contains(t, response.Speech, "Vacuum, S-A-6 awake, bullseye ")
	assert.True(t, strings.HasSuffix(response.Speech, ", tight."))

	response = c.ComposeEmitter("Vacuum", "SA-6", "asleep", "loose", bullseye)
	assert.Equal(t, "Vacuum, SA-6 asleep, bullseye 304/60, loose.", response.Subtitle)
}

func TestSpellReportingName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "S-A-6", spellReportingName("SA-6"))
	assert.Equal(t, "S-A-11", spellReportingName("SA-11"))
	assert.Equal(t, "H-Q-7", spellReportingName("HQ-7"))
	assert.Equal(t, "Hawk", spellReportingName("Hawk"))
	assert.Equal(t, "early warning radar", spellReportingName("early warning radar"))
}
