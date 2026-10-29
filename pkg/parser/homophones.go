package parser

import (
	"slices"
	"strings"
	"unicode"
)

// Fixes for words Whisper writes in place of a callsign's digits, a controller's name or an SA
// designator. Found by speaking RetLab's calls through Whisper on radio-quality audio (2026-10-29).

// romanDigits are Roman numerals Whisper writes for a callsign's number: "Chevy II".
var romanDigits = map[string]string{"ii": "2", "iii": "3", "iv": "4"}

// controllerHomophones are what Whisper writes for a package controller's name at the start of a
// call. Only the first word is replaced: "amble" stays a word anywhere else.
var controllerHomophones = map[string]string{"amble": anvil, "envil": anvil}

const anvil = "anvil"

// fixHeardWords corrects a normalized call's words. wake is true for a word that names the GCI,
// a controller or a speaker.
func fixHeardWords(text string, wake func(string) bool) string {
	fields := strings.Fields(text)
	if len(fields) > 0 {
		if c, ok := controllerHomophones[fields[0]]; ok {
			fields[0] = c
		}
	}
	for i := 1; i < len(fields); i++ {
		prev, f := fields[i-1], fields[i]
		if !isWord(prev) || wake(prev) {
			continue
		}
		if d, ok := romanDigits[f]; ok {
			fields[i] = d
		} else if f == "ford" && (i+1 == len(fields) || !isAllDigits(fields[i+1])) {
			// "Dodge four" written "dodge ford"; "Chainsaw, Ford 9 1" keeps its name.
			fields[i] = "4"
		}
	}
	return strings.Join(splitDesignators(fields), " ")
}

// splitDesignators splits an SA or HQ number run into its bearing: "sa 6180" is SA-6, 180, and
// "sar 6" is SA-6.
func splitDesignators(fields []string) []string {
	out := make([]string, 0, len(fields)+1)
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if f == "sar" && i+1 < len(fields) && isAllDigits(fields[i+1]) {
			f = "sa"
		}
		out = append(out, f)
		if (f == "sa" || f == "hq") && i+1 < len(fields) {
			next := fields[i+1]
			cut := len(next) - 3
			if isAllDigits(next) && cut > 0 && cut <= maxDesignatorDigits && next[cut:] <= "360" {
				out = append(out, next[:cut], next[cut:])
				i++
			}
		}
	}
	return out
}

// isWakeWord is true for a word that names the GCI, a package controller or a speaker.
func (p *Parser) isWakeWord(w string) bool {
	for _, phrase := range p.wakePhrases() {
		if strings.EqualFold(strings.ReplaceAll(phrase, " ", ""), w) {
			return true
		}
	}
	return slices.Contains(p.speakerList(), w)
}

func isWord(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return s != ""
}
