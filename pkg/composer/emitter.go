package composer

import (
	"github.com/dharmab/skyeye/pkg/brevity"
)

// ComposeEmitter constructs natural language for an ISR report of a radiating ground emitter, such
// as "Vacuum, new emitter, SA-6, bullseye 0-9-0, 25." A nil bullseye reports the emitter without a
// position.
func (c *Composer) ComposeEmitter(callsign, label string, bullseye *brevity.Bullseye) NaturalLanguageResponse {
	response := NaturalLanguageResponse{
		Subtitle: callsign + ", new emitter, " + label,
		Speech:   callsign + ", new emitter, " + label,
	}
	if bullseye == nil {
		response.Subtitle += "."
		response.Speech += "."
		return response
	}
	position := c.composeBullseye(bullseye)
	response.Subtitle += ", " + position.Subtitle + "."
	response.Speech += ", " + position.Speech + "."
	return response
}
