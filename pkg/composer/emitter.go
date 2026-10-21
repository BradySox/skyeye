package composer

import (
	"regexp"
	"strings"

	"github.com/dharmab/skyeye/pkg/brevity"
)

// reportingName matches a reporting name such as "SA-6" or "HQ-7".
var reportingName = regexp.MustCompile(`\b([A-Z]{2,3})-(\d+)`)

// spellReportingName makes the speech engine read the letters one by one: Piper says "SA-6" as
// "sah six" but "S-A-6" as "ess ay six".
func spellReportingName(label string) string {
	return reportingName.ReplaceAllStringFunc(label, func(name string) string {
		letters, number, _ := strings.Cut(name, "-")
		return strings.Join(strings.Split(letters, ""), "-") + "-" + number
	})
}

// ComposeEmitter constructs natural language for an ISR report of a radiating ground emitter, such
// as "Vacuum, new emitter, SA-6, bullseye 0-9-0, 25." A nil bullseye reports the emitter without a
// position.
func (c *Composer) ComposeEmitter(callsign, label string, bullseye *brevity.Bullseye) NaturalLanguageResponse {
	response := NaturalLanguageResponse{
		Subtitle: callsign + ", new emitter, " + label,
		Speech:   callsign + ", new emitter, " + spellReportingName(label),
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
