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

// ComposeEmitter constructs natural language for an ISR report of a ground emitter, such as
// "Vacuum, SA-6 awake, bullseye 3-0-4, 60, tight." With no status it is the older "Vacuum, new
// emitter, SA-6, bullseye 0-9-0, 25." A nil bullseye reports the emitter without a position; an
// empty fix says nothing about the fix.
func (c *Composer) ComposeEmitter(callsign, label, status, fix string, bullseye *brevity.Bullseye) NaturalLanguageResponse {
	response := NaturalLanguageResponse{
		Subtitle: callsign + ", new emitter, " + label,
		Speech:   callsign + ", new emitter, " + spellReportingName(label),
	}
	if status != "" {
		response.Subtitle = callsign + ", " + label + " " + status
		response.Speech = callsign + ", " + spellReportingName(label) + " " + status
	}
	if bullseye != nil {
		position := c.composeBullseye(bullseye)
		response.Subtitle += ", " + position.Subtitle
		response.Speech += ", " + position.Speech
	}
	if fix != "" {
		response.Subtitle += ", " + fix
		response.Speech += ", " + fix
	}
	response.Subtitle += "."
	response.Speech += "."
	return response
}
