package composer

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dharmab/skyeye/pkg/brevity"
)

// mixedCase writes a callsign as a name: "ford 9 1" and "FORD 9 1" are "Ford 9 1". The text-to-speech
// voices spell out a short word in capitals ("U-Z-I"). A word already in mixed case is kept.
func mixedCase(callsign string) string {
	words := strings.Fields(callsign)
	for i, word := range words {
		if word != strings.ToLower(word) && word != strings.ToUpper(word) {
			continue
		}
		lower := strings.ToLower(word)
		r, size := utf8.DecodeRuneInString(lower)
		words[i] = string(unicode.ToUpper(r)) + lower[size:]
	}
	return strings.Join(words, " ")
}

func (*Composer) composeCallsigns(callsigns ...string) string {
	for i, callsign := range callsigns {
		if callsign != brevity.LastCaller {
			callsigns[i] = mixedCase(callsign)
		}
	}
	if len(callsigns) == 1 {
		return callsigns[0]
	}
	return strings.Join(callsigns, ", ")
}
