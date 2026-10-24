package parser

import (
	"slices"
	"strings"

	"github.com/dharmab/numwords"
	"github.com/dharmab/skyeye/pkg/brevity"
)

// The way pilots actually talk: the controller named after the callsign ("Ford 9, Image, say
// picture"), extra numbers after the callsign ("checking in, angels 20"), a flight size ("two
// ship"), and a bare "ready" answering a package roll call.

// maxCallsignWordsBeforeController is how many words a pilot callsign may take when the pilot says
// it before the controller's: "Ford 9 1, Image".
const maxCallsignWordsBeforeController = 4

// maxControllerWords is how many words a heard controller callsign may take: "any face".
const maxControllerWords = 3

// niner is the radio word for 9.
const niner = "niner"

// isDigitHomophone is a word Whisper writes for a digit, read as a digit only after one.
func isDigitHomophone(word string) bool {
	_, ok := atcCallsignHomophones[word]
	return ok || word == niner
}

// isCallsignEnd is a word after a number that is never part of a callsign: a distance or a flight
// size.
func isCallsignEnd(word string) bool {
	return distanceWords[word] || shipWords[word]
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// leadingCallsign keeps the words of a normalized call up to the end of the pilot callsign's
// digits: "ford 9 1 check in angels 20" is "ford 9 1". A number after the callsign's digits that is
// written apart ("ford 9 1 10 miles"), or is a distance or a flight size ("ford 9 2 ship"), is not
// read into it. Text with no digits is returned whole.
func leadingCallsign(text string) string {
	fields := strings.Fields(text)
	digits := 0
	for i, f := range fields {
		digit := isAllDigits(f) || (digits > 0 && isDigitHomophone(f))
		if !digit {
			if digits == 1 && isFillerBetweenDigits(fields, i) {
				// "1 on 1": a short word Whisper put between the flight and ship numbers.
				continue
			}
			if digits > 0 {
				return strings.Join(fields[:i], " ")
			}
			continue
		}
		if digits > 0 && (len(f) > 1 || (i+1 < len(fields) && isCallsignEnd(fields[i+1]))) {
			return strings.Join(fields[:i], " ")
		}
		digits++
	}
	return text
}

// isFillerBetweenDigits is true for a word of at most two letters between two single digits, the
// second of them not a distance or a flight size.
func isFillerBetweenDigits(fields []string, i int) bool {
	if len(fields[i]) > 2 || i == 0 || len(fields[i-1]) != 1 || i+1 >= len(fields) {
		return false
	}
	next := fields[i+1]
	if len(next) != 1 || !isAllDigits(next) {
		return false
	}
	return i+2 >= len(fields) || !isCallsignEnd(fields[i+2])
}

// hasDigits is true if the words hold a number, as a pilot callsign does.
func hasDigits(fields []string) bool {
	return slices.ContainsFunc(strings.Fields(numwords.ParseString(strings.Join(fields, " "))), isAllDigits)
}

// findControllerAfterCallsign finds the controller callsign said after the pilot's: "Ford 9,
// Image, say picture". Only a pilot callsign with a number may come before it. Returns the heard
// controller callsign, the wake phrase it matched, the pilot callsign's words, and whether it was
// found.
func (p *Parser) findControllerAfterCallsign(fields []string) (heard string, matched string, pilot string, ok bool) {
	phrases := p.wakePhrases()
	for start := 1; start <= maxCallsignWordsBeforeController && start < len(fields); start++ {
		if !hasDigits(fields[:start]) || hasDigits(fields[start:start+1]) {
			// The controller callsign starts after the pilot's last digit.
			continue
		}
		for n := 1; n <= maxControllerWords && start+n <= len(fields); n++ {
			candidate := strings.Join(fields[start:start+n], " ")
			for _, wakePhrase := range phrases {
				if wakePhrase != "" && isSimilar(candidate, strings.ToLower(wakePhrase)) {
					return candidate, strings.ToLower(wakePhrase), strings.Join(fields[:start], " "), true
				}
			}
		}
	}
	return "", "", "", false
}

// callsignLeadIns are words a pilot says around the callsign and the request: "Chainsaw, say
// picture", "uh, request picture", "this is Ford 1 1". They are never a pilot callsign.
var callsignLeadIns = map[string]bool{
	"uh": true, "uhh": true, "um": true, "umm": true, "er": true, "erm": true, "ah": true,
	wordSay: true, wordRequest: true, wordRequesting: true, "give": true, "me": true, "send": true,
	"whats": true, "what": true, "how": true, "about": true, wordThis: true, "is": true,
	"the": true, "your": true, "please": true,
}

const (
	wordRequesting = "requesting"
	wordThis       = "this"
)

// trimLeadIns drops lead-in words from both ends of the words heard as the pilot callsign.
func trimLeadIns(text string) string {
	fields := strings.Fields(text)
	for len(fields) > 0 && callsignLeadIns[fields[0]] {
		fields = fields[1:]
	}
	for len(fields) > 0 && callsignLeadIns[fields[len(fields)-1]] {
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, " ")
}

// sayHomophones are what Whisper writes for "say". They count only before a request word: "safe
// picture".
var sayHomophones = map[string]bool{"safe": true}

// fixSayHomophones rewrites a homophone of "say" that comes right before a request word.
func fixSayHomophones(fields []string) []string {
	for i := 0; i+1 < len(fields); i++ {
		if !sayHomophones[fields[i]] {
			continue
		}
		if _, _, ok := findRequestWord(fields[i+1 : i+2]); ok {
			fields[i] = wordSay
		}
	}
	return fields
}

// findControllerAfterFlightName finds the controller callsign said after a flight name with no
// number: "Chevy, Chainsaw, say picture". Only the lead-ins may follow the controller, so the call
// is a bare request. Returns the heard controller callsign, the wake phrase it matched, the flight
// name, and whether it was found.
func (p *Parser) findControllerAfterFlightName(fields []string) (heard string, matched string, pilot string, ok bool) {
	if len(fields) < 2 || callsignLeadIns[fields[0]] || hasDigits(fields[:1]) {
		return "", "", "", false
	}
	phrases := p.wakePhrases()
	for n := 1; n <= maxControllerWords && 1+n <= len(fields); n++ {
		if trimLeadIns(strings.Join(fields[1+n:], " ")) != "" {
			continue
		}
		candidate := strings.Join(fields[1:1+n], " ")
		for _, wakePhrase := range phrases {
			if wakePhrase != "" && isSimilar(candidate, strings.ToLower(wakePhrase)) {
				return candidate, strings.ToLower(wakePhrase), fields[0], true
			}
		}
	}
	return "", "", "", false
}

// packageAcknowledgements answer a package controller and need no reply: "Ford 9, copy".
var packageAcknowledgements = map[string]bool{
	"copy": true, "copies": true, "wilco": true, "roger": true, "pushing": true, "egressing": true,
}

// airbornePhrases report a flight off the runway: "Chevy 2, flight is airborne", "wheels up".
var airbornePhrases = []string{"airborne", "wheels up", "off the ground", "departing"}

// isAirborneReport is true if the call reports the flight airborne.
func isAirborneReport(tx string) bool {
	padded := " " + tx + " "
	for _, phrase := range airbornePhrases {
		if strings.Contains(padded, " "+phrase+" ") {
			return true
		}
	}
	return false
}

// parsePackageCallWithoutRequestWord reads a call to a package controller that names no request
// word. "Check in" (and its aliases) or a bare "ready", which answers the roll call, is a check-in;
// "airborne" or "wheels up" is an airborne report; an acknowledgement needs no reply; anything else asks the controller to say again, in its own
// voice. Returns nil for an acknowledgement.
func parsePackageCallWithoutRequestWord(tx, pilotCallsign, controller string) any {
	fields := strings.Fields(tx)
	if strings.Contains(tx, checkIn) {
		return &brevity.PackageRequest{Callsign: pilotCallsign, Controller: controller, Kind: brevity.PackageCheckIn}
	}
	if isAirborneReport(tx) {
		return &brevity.PackageRequest{Callsign: pilotCallsign, Controller: controller, Kind: brevity.PackageAirborne}
	}
	if strings.Contains(" "+tx+" ", " ready ") {
		return &brevity.PackageRequest{Callsign: pilotCallsign, Controller: controller, Kind: brevity.PackageCheckIn}
	}
	for _, f := range fields {
		if packageAcknowledgements[f] {
			return nil
		}
	}
	return &brevity.PackageRequest{Callsign: pilotCallsign, Controller: controller, Kind: brevity.PackageUnknown}
}
