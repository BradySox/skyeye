package composer

import (
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
	response := c.ComposeEmitter("Vacuum", "SA-6", bullseye)
	assert.Equal(t, "Vacuum, new emitter, SA-6, bullseye 090/25.", response.Subtitle)
	assert.Contains(t, response.Speech, "Vacuum, new emitter, SA-6, bullseye ")

	response = c.ComposeEmitter("Vacuum", "SA-6", nil)
	assert.Equal(t, "Vacuum, new emitter, SA-6.", response.Subtitle)
}
