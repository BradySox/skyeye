package parser

import (
	"slices"
	"strings"
	"unicode"

	"github.com/dharmab/skyeye/pkg/brevity"
)

// ATC calls are parsed apart from GCI calls: they are heard only on airfield frequencies, open with
// the controller's position instead of a GCI callsign, and a readback ends with the callsign.

// Controller positions a pilot may call.
const (
	positionGround    = "ground"
	positionTower     = "tower"
	positionApproach  = "approach"
	positionDeparture = "departure"
	positionArrival   = "arrival"
)

var atcPositions = []string{positionGround, positionTower, positionApproach, positionDeparture, positionArrival}

// maxFieldNameWords is the most words an airfield name may take before the position.
const maxFieldNameWords = 4

var atcDigitWords = map[string]string{
	"zero": "0", "one": "1", "two": "2", "three": "3", "four": "4",
	"five": "5", "six": "6", "seven": "7", "eight": "8", "nine": "9", "niner": "9",
}

// atcCallsignHomophones are words Whisper writes for digits, accepted only inside a callsign.
var atcCallsignHomophones = map[string]string{
	"won": "1", "to": "2", "too": "2", "tu": "2", "for": "4", "fore": "4",
	"free": "3", "tree": "3", "ate": "8",
}

// notCallsignWords open a request, never a callsign: "ready for takeoff" is not "Ready 4".
var notCallsignWords = map[string]bool{
	"request": true, "requesting": true, "ready": true, "holding": true, "taxi": true,
	"inbound": true, "clear": true, "airborne": true, "say": true, "radio": true,
}

// distanceWords follow a distance, which is never part of a callsign.
var distanceWords = map[string]bool{"mile": true, "miles": true, "out": true}

// reservedCallsignWords are clearance words, never a callsign's name: in "hold short 28" the
// callsign is not "Short 2-8".
var reservedCallsignWords = map[string]bool{
	"runway": true, "short": true, "hold": true, "taxi": true, "squawk": true, "cleared": true,
	"tower": true, "ground": true, "takeoff": true, "contact": true, "wind": true, "altimeter": true,
}

// shipWords follow a flight's size: in "Viper 1, two ship" the 2 is not ship 2.
var shipWords = map[string]bool{"ship": true, "ships": true}

// decimalMark is the token for a decimal point in a spoken frequency.
const decimalMark = "."

// numberBreak is the token between two numbers written apart, as in "Viper 1, 10 miles", so the
// distance is never read into the callsign.
const numberBreak = ","

// atcTokens splits a call into words, single digits and decimal points. Commas inside a number
// ("3,000") are dropped; "decimal" and "point" become decimal points. Two numbers written apart
// by punctuation ("9, 10") get a numberBreak between them.
func atcTokens(tx string) []string {
	tx = strings.ToLower(tx)
	tx, _, _ = strings.Cut(tx, "|")
	runes := []rune(tx)
	var tokens []string
	var word strings.Builder
	broken := false
	emit := func(t string) {
		if broken && isDigitToken(t) && len(tokens) > 0 && isDigitToken(tokens[len(tokens)-1]) {
			tokens = append(tokens, numberBreak)
		}
		broken = false
		tokens = append(tokens, t)
	}
	flush := func() {
		if word.Len() == 0 {
			return
		}
		w := word.String()
		word.Reset()
		switch {
		case atcDigitWords[w] != "":
			emit(atcDigitWords[w])
		case w == "decimal" || w == "point":
			emit(decimalMark)
		default:
			emit(w)
		}
	}
	isDigitAt := func(i int) bool { return i >= 0 && i < len(runes) && unicode.IsDigit(runes[i]) }
	for i, r := range runes {
		switch {
		case unicode.IsDigit(r):
			flush()
			emit(string(r))
		case unicode.IsLetter(r):
			word.WriteRune(unicode.ToLower(r))
		case r == ',' && isDigitAt(i-1) && isDigitAt(i+1):
			// A thousands separator: keep the digits together.
		case r == '.' && isDigitAt(i-1) && isDigitAt(i+1):
			flush()
			emit(decimalMark)
		default:
			flush()
			if !unicode.IsSpace(r) && r != '-' {
				broken = true
			}
		}
	}
	flush()
	return tokens
}

// spokenText is the call as SkyEye heard it, without numberBreaks.
func spokenText(tokens []string) string {
	words := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t != numberBreak {
			words = append(words, t)
		}
	}
	return strings.Join(words, " ")
}

func isDigitToken(t string) bool {
	return len(t) == 1 && t[0] >= '0' && t[0] <= '9'
}

// maxCallsignDigits is the flight and ship number: DCS callsigns are a name and two digits, so a
// number spoken right after the callsign ("Viper 1-1, 5 miles") is not read into it.
const maxCallsignDigits = 2

// isCallsignName is false for a word that opens a request or is part of a clearance.
func isCallsignName(word string) bool {
	return !notCallsignWords[word] && !reservedCallsignWords[word]
}

// digitRun is how many digit tokens start at tokens[i].
func digitRun(tokens []string, i int) int {
	n := 0
	for i+n < len(tokens) && isDigitToken(tokens[i+n]) {
		n++
	}
	return n
}

// callsignSpan finds a callsign starting at tokens[start]: one or two name words, then up to two
// digits (or digit homophones). It returns the callsign and the index just past it. The digits stop
// at the first word after them, a flight size ("two ship") and a distance ("10 miles").
func callsignSpan(tokens []string, start int) (string, int, bool) {
	i := start
	for i+1 < len(tokens) && tokens[i] == "this" && tokens[i+1] == "is" {
		i += 2
	}
	var name []string
	for i < len(tokens) && len(name) < 2 && !isDigitToken(tokens[i]) && tokens[i] != decimalMark && tokens[i] != numberBreak {
		if _, ok := atcCallsignHomophones[tokens[i]]; ok && len(name) > 0 {
			break
		}
		name = append(name, tokens[i])
		i++
	}
	if len(name) == 0 {
		return "", start, false
	}
	for _, w := range name {
		if !isCallsignName(w) {
			return "", start, false
		}
	}
	var digits []string
	for i < len(tokens) && len(digits) < maxCallsignDigits {
		t := tokens[i]
		if d, ok := atcCallsignHomophones[t]; ok {
			t = d
		}
		if !isDigitToken(t) {
			break
		}
		if len(digits) > 0 {
			// "Viper 1, two ship": the flight's size, not ship 2.
			if i+1 < len(tokens) && shipWords[tokens[i+1]] {
				break
			}
			// "Viper 1, 5 miles" or "Viper 1 10 miles": a distance of one or two digits.
			if run := digitRun(tokens, i); run > 0 && i+run < len(tokens) && distanceWords[tokens[i+run]] && run <= 2 {
				break
			} else if run > 2 && i+run < len(tokens) && distanceWords[tokens[i+run]] {
				// "Viper 1 1 12 miles": the ship, then a two-digit distance.
				digits = append(digits, t)
				i++
				break
			}
		}
		digits = append(digits, t)
		i++
	}
	if len(digits) == 0 {
		return "", start, false
	}
	return strings.Join(append(name, digits...), " "), i, true
}

// tailCallsign finds the callsign a readback ends with: a name word, then one or two digits.
func tailCallsign(tokens []string) (string, int, bool) {
	end := len(tokens)
	i := end
	digits := 0
	for i > 0 && digits < maxCallsignDigits {
		t := tokens[i-1]
		if d, ok := atcCallsignHomophones[t]; ok {
			t = d
		}
		if !isDigitToken(t) {
			break
		}
		digits++
		i--
	}
	if digits == 0 || i == 0 || isDigitToken(tokens[i-1]) || tokens[i-1] == decimalMark || tokens[i-1] == numberBreak {
		return "", end, false
	}
	callsign, _, ok := callsignSpan(tokens, i-1)
	return callsign, i - 1, ok
}

// atcNumbers joins runs of digits, with any decimal point inside them, into numbers: "2 7 0 . 4 5"
// is "270.45". "Thousand" and "hundred" scale the number before them.
func atcNumbers(tokens []string) []string {
	var numbers []string
	var current strings.Builder
	flush := func() {
		n := strings.TrimSuffix(current.String(), decimalMark)
		current.Reset()
		if n != "" && n != decimalMark {
			numbers = append(numbers, n)
		}
	}
	for _, t := range tokens {
		switch {
		case isDigitToken(t):
			current.WriteString(t)
		case t == decimalMark && current.Len() > 0 && !strings.Contains(current.String(), decimalMark):
			current.WriteString(t)
		case (t == "thousand" || t == "hundred") && current.Len() > 0:
			if t == "thousand" {
				current.WriteString("000")
			} else {
				current.WriteString("00")
			}
			flush()
		default:
			flush()
		}
	}
	flush()
	return numbers
}

func hasToken(tokens []string, words ...string) bool {
	for _, t := range tokens {
		if slices.Contains(words, t) {
			return true
		}
	}
	return false
}

func hasPhrase(tokens []string, phrase string) bool {
	return strings.Contains(" "+strings.Join(tokens, " ")+" ", " "+phrase+" ")
}

// atcKind reads the request in the words after the callsign.
func atcKind(tokens []string) (brevity.ATCRequestKind, string) {
	switch {
	case hasToken(tokens, "mayday", "emergency") || hasPhrase(tokens, "pan pan"): //nolint:dupword // the call is "pan pan"
		return brevity.ATCEmergency, ""
	case isRadioCheck(tokens):
		return brevity.ATCRadioCheck, ""
	case hasPhrase(tokens, "say again") || hasToken(tokens, "repeat"):
		return brevity.ATCSayAgain, ""
	case hasToken(tokens, "clear", "clearing", "vacated"):
		return brevity.ATCClear, ""
	case hasToken(tokens, "start", "startup", "engine", "engines"):
		return brevity.ATCStartup, ""
	case hasToken(tokens, "taxi", "taxis"):
		return brevity.ATCTaxi, ""
	case hasToken(tokens, "holding", "ready", "takeoff", "departure") || hasPhrase(tokens, "take off"):
		return brevity.ATCReady, ""
	case isAirborne(tokens):
		return brevity.ATCAirborne, ""
	case hasToken(tokens, "overhead", "break"):
		return brevity.ATCInbound, "overhead"
	case hasToken(tokens, "inbound", "mile", "miles", "initial", "landing", "full", "final") ||
		hasPhrase(tokens, "straight in") || hasPhrase(tokens, "visual approach"):
		return brevity.ATCInbound, ""
	}
	return brevity.ATCUnknown, ""
}

// radioCheckPhrases are the ways a pilot asks how he is heard.
var radioCheckPhrases = []string{
	"radio check", "comm check", "comms check", "com check", "mic check", "mike check",
	"how do you read", "how copy", "how do you copy",
}

func isRadioCheck(tokens []string) bool {
	for _, phrase := range radioCheckPhrases {
		if hasPhrase(tokens, phrase) {
			return true
		}
	}
	return false
}

// isAirborne is a departure report: "airborne", "outbound", "departing", "wheels up", "switching".
func isAirborne(tokens []string) bool {
	return hasToken(tokens, "airborne", "outbound", "departing", "switching") ||
		hasPhrase(tokens, "wheels up") || hasPhrase(tokens, "frequency change")
}

// isReadback is true for a call that repeats a clearance back: "Viper 1-1, cleared for takeoff",
// "Viper 1-1, squawk 4101, hold short 25". A request in it makes it a request instead.
func isReadback(tokens []string) bool {
	if hasToken(tokens, "request", "requesting", "ready", "holding", "inbound", "airborne") {
		return false
	}
	return hasToken(tokens, "cleared", "squawk", "wilco", "roger", "copy", "copies") || hasPhrase(tokens, "hold short")
}

// isEmergencyOpening is "mayday" or "pan", the words an emergency call opens with.
func isEmergencyOpening(token string) bool {
	return isSimilar("mayday", token) || token == "pan"
}

// findPosition finds the controller position a call opens with. Only words, at most a field name's
// worth, may come before it, so a readback such as "3000, golf, tower at 5" is not a call to tower.
func findPosition(tokens []string) (string, int, bool) {
	for i := 0; i < len(tokens) && i <= maxFieldNameWords; i++ {
		if isDigitToken(tokens[i]) || tokens[i] == decimalMark {
			return "", 0, false
		}
		for _, position := range atcPositions {
			if isSimilar(position, tokens[i]) {
				return position, i, true
			}
		}
	}
	return "", 0, false
}

func withoutSpan(tokens []string, from, to int) []string {
	out := make([]string, 0, len(tokens))
	out = append(out, tokens[:from]...)
	return append(out, tokens[to:]...)
}

// ParseATC reads a call heard on an airfield frequency. A call opens with the controller ("Kobuleti
// Ground", or just "Tower"), then the callsign and the request; a mayday opens with "mayday"; a
// readback is the clearance's items, then the callsign. Returns nil for chatter with no callsign.
//
// Loose parsing is for the mission's Casual radio discipline: a call may leave out the station
// ("Viper 1-1, request taxi") or the callsign ("Tower, ready for takeoff"), and the mission
// works out who called from the frequency.
func ParseATC(tx string, loose bool) *brevity.ATCRequest {
	if len(tx) > maxInputLength {
		tx = tx[:maxInputLength]
	}
	tokens := atcTokens(tx)
	if len(tokens) == 0 {
		return nil
	}
	text := spokenText(tokens)

	if isEmergencyOpening(tokens[0]) {
		start := 1
		for start < len(tokens) && isEmergencyOpening(tokens[start]) {
			start++
		}
		callsign, end, ok := callsignSpan(tokens, start)
		if !ok {
			return nil
		}
		return &brevity.ATCRequest{
			Callsign: callsign,
			Kind:     brevity.ATCEmergency,
			Numbers:  atcNumbers(tokens[end:]),
			Text:     text,
		}
	}

	if position, idx, ok := findPosition(tokens); ok {
		callsign, end, ok := callsignSpan(tokens, idx+1)
		if !ok {
			if !loose {
				return nil
			}
			end = idx + 1
		}
		rest := tokens[end:]
		kind, detail := atcKind(rest)
		if hasToken(rest, "cleared") && !hasToken(rest, "request", "requesting") {
			// "Tower, Viper 1-1, cleared for takeoff 25" reads the clearance back.
			kind, detail = brevity.ATCReadback, ""
		}
		return &brevity.ATCRequest{
			Callsign: callsign,
			Position: position,
			Kind:     kind,
			Detail:   detail,
			Numbers:  atcNumbers(rest),
			Text:     text,
		}
	}

	callsign, start, ok := tailCallsign(tokens)
	var items []string
	if ok {
		items = withoutSpan(tokens, start, len(tokens))
	} else if head, end, found := callsignSpan(tokens, 0); found && isReadback(tokens[end:]) {
		// The callsign first: "Viper 1-1, cleared for takeoff 25".
		callsign, items = head, tokens[end:]
	} else {
		if loose {
			return looseRequest(tokens, text)
		}
		return nil
	}
	kind := brevity.ATCReadback
	if k, _ := atcKind(items); k == brevity.ATCRadioCheck || k == brevity.ATCSayAgain {
		kind = k
	}
	return &brevity.ATCRequest{
		Callsign: callsign,
		Kind:     kind,
		Numbers:  atcNumbers(items),
		Text:     text,
	}
}

// looseRequest reads a call with no station: the callsign first if there is one, then the request.
// A call that names no request SkyEye knows is chatter.
func looseRequest(tokens []string, text string) *brevity.ATCRequest {
	callsign, end, ok := callsignSpan(tokens, 0)
	if !ok {
		end = 0
	}
	rest := tokens[end:]
	kind, detail := atcKind(rest)
	if kind == brevity.ATCUnknown {
		return nil
	}
	return &brevity.ATCRequest{
		Callsign: callsign,
		Kind:     kind,
		Detail:   detail,
		Numbers:  atcNumbers(rest),
		Text:     text,
	}
}
