// Package tuning holds the pure-Go settings of the speech recognizers: the initial prompt that
// steers recognition toward the words pilots say, and the local recognizer's thread count.
package tuning

import (
	"fmt"
	"strings"
)

// MaxPromptChars keeps the prompt inside Whisper's 224-token prompt window. whisper.cpp keeps the
// end of a longer prompt and drops its start; English runs about four characters a token, and
// callsigns and place names tokenize worse, so the budget is well under 224 * 4.
const MaxPromptChars = 600

// DefaultThreads is the default CPU thread count for local recognition. whisper.cpp's Go binding
// uses every logical CPU, which on a 28-thread desktop running DCS slows recognition down.
const DefaultThreads = 8

// atcWords are the words of a Ground or Tower call. No squawk: RetLab's ATC gives no codes.
const atcWords = "Ground, Tower, request taxi, holding short, ready for takeoff, runway, inbound, outbound, full stop."

// packageWords are the words of a package call.
const packageWords = "Check in, checking in, package, on station."

// Vocabulary is what the mission tells SkyEye that pilots will say.
type Vocabulary struct {
	// Callsign is SkyEye's own GCI callsign for the mission. Empty keeps the configured one.
	Callsign string
	// Callsigns are the other callsigns pilots call or call as: controllers, packages, flights.
	Callsigns []string
	// Fields are the airfields the mission runs ATC for.
	Fields []string
}

// IsZero is true when the mission has told SkyEye nothing.
func (v Vocabulary) IsZero() bool {
	return v.Callsign == "" && len(v.Callsigns) == 0 && len(v.Fields) == 0
}

// Prompt builds the initial prompt from SkyEye's callsign, the configured locations and the
// mission's vocabulary, within MaxPromptChars. The mission's words come last, nearest the
// transcript, and locations are dropped first when the budget runs out.
func Prompt(callsign string, locations []string, vocab Vocabulary) string {
	if vocab.Callsign != "" {
		callsign = vocab.Callsign
	}
	base := fmt.Sprintf("Either ANYFACE or %s, PILOT CALLSIGN, DIGITS, one of 'RADIO' 'ALPHA' 'BOGEY' 'PICTURE' 'DECLARE' 'SNAPLOCK' 'SPIKED', ARGUMENTS such as BULLSEYE, BRAA, numbers or digits.", callsign)

	var tail []string
	if len(vocab.Callsigns) > 0 {
		tail = append(tail, packageWords)
	}
	if len(vocab.Fields) > 0 {
		tail = append(tail, atcWords)
	}
	budget := MaxPromptChars - len(base) - joinedLen(tail)
	callsigns, budget := listSentence("Callsigns: ", unique(vocab.Callsigns), budget)
	fields, budget := listSentence("Airfields: ", unique(vocab.Fields), budget)
	places, _ := listSentence("Locations: ", unique(locations), budget)

	parts := []string{base}
	for _, s := range []string{places, strings.Join(tail, " "), callsigns, fields} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}

// joinedLen is the length of the parts joined by spaces, plus the space before them.
func joinedLen(parts []string) int {
	n := 0
	for _, p := range parts {
		n += len(p) + 1
	}
	return n
}

// listSentence is "Label: a, b, c." with as many items as fit in budget, counting the space
// before it, and the budget left.
func listSentence(label string, items []string, budget int) (string, int) {
	if len(items) == 0 {
		return "", budget
	}
	var b strings.Builder
	for _, item := range items {
		sep := ", "
		if b.Len() == 0 {
			sep = label
		}
		// One for the space before the sentence, one for its period.
		if b.Len()+len(sep)+len(item)+2 > budget {
			break
		}
		b.WriteString(sep)
		b.WriteString(item)
	}
	if b.Len() == 0 {
		return "", budget
	}
	b.WriteString(".")
	return b.String(), budget - b.Len() - 1
}

// unique drops blank and repeated items, ignoring case and keeping the first spelling.
func unique(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		key := strings.ToLower(item)
		if item == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

// Threads is the recognizer's thread count: the requested count, or DefaultThreads when it is
// zero or less, never more than the machine's logical CPUs and never less than one.
func Threads(requested, numCPU int) uint {
	n := requested
	if n <= 0 {
		n = DefaultThreads
	}
	n = min(n, numCPU)
	return uint(max(n, 1)) //nolint:gosec // n is at least one
}
