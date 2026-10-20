package parser

import (
	"slices"
	"strings"
	"unicode"

	"github.com/dharmab/skyeye/pkg/brevity"
)

// ATC calls are parsed apart from GCI calls: they are heard only on airfield frequencies, open with
// the controller's position instead of a GCI callsign, and a readback ends with the callsign.

// atcPositions are the controller positions a pilot may call.
var atcPositions = []string{"ground", "tower", "approach", "departure", "arrival"}

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

// decimalMark is the token for a decimal point in a spoken frequency.
const decimalMark = "."

// atcTokens splits a call into words, single digits and decimal points. Commas inside a number
// ("3,000") are dropped; "decimal" and "point" become decimal points.
func atcTokens(tx string) []string {
	tx = strings.ToLower(tx)
	tx, _, _ = strings.Cut(tx, "|")
	runes := []rune(tx)
	var tokens []string
	var word strings.Builder
	flush := func() {
		if word.Len() == 0 {
			return
		}
		w := word.String()
		word.Reset()
		switch {
		case atcDigitWords[w] != "":
			tokens = append(tokens, atcDigitWords[w])
		case w == "decimal" || w == "point":
			tokens = append(tokens, decimalMark)
		default:
			tokens = append(tokens, w)
		}
	}
	isDigitAt := func(i int) bool { return i >= 0 && i < len(runes) && unicode.IsDigit(runes[i]) }
	for i, r := range runes {
		switch {
		case unicode.IsDigit(r):
			flush()
			tokens = append(tokens, string(r))
		case unicode.IsLetter(r):
			word.WriteRune(unicode.ToLower(r))
		case r == ',' && isDigitAt(i-1) && isDigitAt(i+1):
			// A thousands separator: keep the digits together.
		case r == '.' && isDigitAt(i-1) && isDigitAt(i+1):
			flush()
			tokens = append(tokens, decimalMark)
		default:
			flush()
		}
	}
	flush()
	return tokens
}

func isDigitToken(t string) bool {
	return len(t) == 1 && t[0] >= '0' && t[0] <= '9'
}

// maxCallsignDigits is the flight and ship number: DCS callsigns are a name and two digits, so a
// number spoken right after the callsign ("Viper 1-1, 5 miles") is not read into it.
const maxCallsignDigits = 2

// callsignSpan finds a callsign starting at tokens[start]: one or two name words, then up to two
// digits (or digit homophones). It returns the callsign and the index just past it.
func callsignSpan(tokens []string, start int) (string, int, bool) {
	i := start
	for i+1 < len(tokens) && tokens[i] == "this" && tokens[i+1] == "is" {
		i += 2
	}
	var name []string
	for i < len(tokens) && len(name) < 2 && !isDigitToken(tokens[i]) && tokens[i] != decimalMark {
		if _, ok := atcCallsignHomophones[tokens[i]]; ok && len(name) > 0 {
			break
		}
		name = append(name, tokens[i])
		i++
	}
	if len(name) == 0 {
		return "", start, false
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
	if digits == 0 || i == 0 || isDigitToken(tokens[i-1]) || tokens[i-1] == decimalMark {
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
	case hasToken(tokens, "mayday", "emergency"):
		return brevity.ATCEmergency, ""
	case hasPhrase(tokens, "radio check"):
		return brevity.ATCRadioCheck, ""
	case hasPhrase(tokens, "say again"):
		return brevity.ATCSayAgain, ""
	case hasToken(tokens, "clear", "clearing", "vacated"):
		return brevity.ATCClear, ""
	case hasToken(tokens, "start", "startup", "engine", "engines"):
		return brevity.ATCStartup, ""
	case hasToken(tokens, "taxi", "taxis"):
		return brevity.ATCTaxi, ""
	case hasToken(tokens, "holding", "ready", "takeoff", "departure") || hasPhrase(tokens, "take off"):
		return brevity.ATCReady, ""
	case hasToken(tokens, "airborne"):
		return brevity.ATCAirborne, ""
	case hasToken(tokens, "overhead", "break"):
		return brevity.ATCInbound, "overhead"
	case hasToken(tokens, "inbound", "miles", "initial", "landing", "full") || hasPhrase(tokens, "straight in"):
		return brevity.ATCInbound, ""
	}
	return brevity.ATCUnknown, ""
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
func ParseATC(tx string) *brevity.ATCRequest {
	if len(tx) > maxInputLength {
		tx = tx[:maxInputLength]
	}
	tokens := atcTokens(tx)
	if len(tokens) == 0 {
		return nil
	}
	text := strings.Join(tokens, " ")

	if isSimilar("mayday", tokens[0]) {
		start := 1
		for start < len(tokens) && isSimilar("mayday", tokens[start]) {
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
			return nil
		}
		rest := tokens[end:]
		kind, detail := atcKind(rest)
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
	if !ok {
		return nil
	}
	items := withoutSpan(tokens, start, len(tokens))
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
