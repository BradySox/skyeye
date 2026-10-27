package parser

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dharmab/numwords"
	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/dharmab/skyeye/pkg/callsigns"
)

// ELINT calls go to a mission speaker such as Vacuum, which knows the radiating radars. They are
// parsed apart from GCI calls so a call to Vacuum never reaches the GCI or a package controller.
// "Picture" stays the GCI's word; Vacuum's words are MUD, ACTIVE, BLANK and SAY AGAIN.

// SetSpeaker adds a mission ELINT speaker's callsign, such as "Vacuum", once.
func (p *Parser) SetSpeaker(callsign string) {
	c := strings.ToLower(strings.ReplaceAll(callsign, " ", ""))
	if c == "" {
		return
	}
	p.packageControllersLock.Lock()
	defer p.packageControllersLock.Unlock()
	if !slices.Contains(p.speakers, c) {
		p.speakers = append(p.speakers, c)
	}
}

func (p *Parser) speakerList() []string {
	p.packageControllersLock.RLock()
	defer p.packageControllersLock.RUnlock()
	return slices.Clone(p.speakers)
}

// maxSpeakerWords is how many words a heard speaker callsign may take: "vac you um".
const maxSpeakerWords = 3

// isSpeaker is the speaker callsign the words make, if any.
func isSpeaker(words []string, speakers []string) (string, bool) {
	candidate := strings.Join(words, "")
	for _, s := range speakers {
		if candidate == s || (len(candidate) >= 4 && isSimilar(candidate, s)) {
			return s, true
		}
	}
	return "", false
}

// matchSpeaker finds a speaker callsign at the start of a call, or after a pilot callsign with a
// number, as the GCI's is found: "Ford 9, Vacuum, say emitters". rest is the call without it.
func matchSpeaker(fields []string, speakers []string) (speaker string, rest []string, ok bool) {
	for i := 0; i < len(fields) && i < maxSpeakerWords; i++ {
		if s, ok := isSpeaker(fields[:i+1], speakers); ok {
			return s, fields[i+1:], true
		}
	}
	for start := 1; start <= maxCallsignWordsBeforeController && start < len(fields); start++ {
		if !hasDigits(fields[:start]) || hasDigits(fields[start:start+1]) {
			continue
		}
		for n := 1; n <= maxSpeakerWords && start+n <= len(fields); n++ {
			if s, ok := isSpeaker(fields[start:start+n], speakers); ok {
				return s, append(slices.Clone(fields[:start]), fields[start+n:]...), true
			}
		}
	}
	return "", nil, false
}

const (
	wordSay     = "say"
	wordRequest = "request"
)

// elintLeadIns are words a pilot says around the callsign and the request: "request target
// status", "say emitters", "uh, give me emitters". They are never part of the callsign.
var elintLeadIns = map[string]bool{
	"uh": true, "uhh": true, "um": true, "umm": true, "er": true, "ah": true, "erm": true,
	wordSay: true, wordRequest: true, wordRequesting: true, "give": true, "me": true, "send": true,
	"whats": true, "what": true, "s": true, "is": true, "how": true, "about": true, wordThis: true,
	"the": true, "your": true, "please": true, "and": true, "with": true, "copy": true,
}

// mudHomophones are what Whisper writes for "mud". They count only when a bearing follows.
var mudHomophones = map[string]bool{"mod": true, "much": true, "mudd": true, "mug": true, "mutt": true, "mad": true}

// bearingLeadIns are words between MUD and its bearing: "mud spike at 270", "mud bearing 270".
var bearingLeadIns = map[string]bool{"at": true, "spike": true, "bearing": true, "is": true, "on": true, "of": true, "uh": true, "um": true}

// radarNames are radar and SAM names a pilot may say beside a mud bearing: "mud Hawk 185".
var radarNames = map[string]bool{
	"sam": true, "radar": true, "hawk": true, "patriot": true, "roland": true, "rapier": true,
	"nasams": true, "kub": true, "buk": true, "osa": true, "tor": true, "strela": true,
	"tunguska": true, "pantsir": true, "gainful": true, "gecko": true, "gadfly": true,
	"grumble": true, "goa": true, "guideline": true, "gladiator": true, "giant": true,
	"gauntlet": true, "grison": true, "greyhound": true, "gopher": true, "grizzly": true,
}

// maxDesignatorDigits is the most digits an SA or HQ number takes: SA-6, SA-23.
const maxDesignatorDigits = 2

// radarWords is how many fields at i name a radar: "sa 6", "s a 6", "hq 7" or "hawk", else 0.
// Whisper splits "SA-6-185" into "sa 6 185", so the number after SA is the system, not the bearing.
func radarWords(fields []string, i int) int {
	if radarNames[fields[i]] {
		return 1
	}
	n := 0
	switch {
	case fields[i] == "sa" || fields[i] == "hq":
		n = 1
	case fields[i] == "s" && i+1 < len(fields) && fields[i+1] == "a":
		n = 2
	default:
		return 0
	}
	if i+n < len(fields) && len(fields[i+n]) <= maxDesignatorDigits {
		if _, err := strconv.Atoi(fields[i+n]); err == nil {
			n++
		}
	}
	return n
}

// parseMudBearing reads a bearing of one to three digits, spoken digit by digit or as one number.
// A radar named before it is skipped: "mud SA-6 185", "mud Hawk 185".
func parseMudBearing(fields []string) (string, bool) {
	bearing, _, ok := mudBearingAt(fields)
	return bearing, ok
}

// mudBearingAt is parseMudBearing, with the index of the first field after the bearing.
func mudBearingAt(fields []string) (bearing string, end int, ok bool) {
	i := 0
	for i < len(fields) {
		skip := radarWords(fields, i)
		if bearingLeadIns[fields[i]] {
			skip = 1
		}
		if skip == 0 {
			break
		}
		i += skip
	}
	var digits strings.Builder
	for ; i < len(fields) && digits.Len() < 3; i++ {
		f := fields[i]
		if f == niner {
			f = "9"
		}
		if _, err := strconv.Atoi(f); err != nil {
			break
		}
		digits.WriteString(f)
	}
	if digits.Len() == 0 || digits.Len() > 3 {
		return "", i, false
	}
	n, err := strconv.Atoi(digits.String())
	if err != nil || n > 360 {
		return "", i, false
	}
	return fmt.Sprintf("%03d", n%360), i, true
}

// MudFollowUpWindow is how long after a mud call with no bearing ("say mud bearing") the same
// caller's bare bearing, such as "1 8 0", still answers it.
const MudFollowUpWindow = 30 * time.Second

type pendingMud struct {
	request brevity.ELINTRequest
	at      time.Time
}

// AwaitMudBearing remembers a mud call heard with no bearing, so caller's next bare bearing is
// taken as that call. caller names the radio and frequency it was heard from.
func (p *Parser) AwaitMudBearing(caller string, request *brevity.ELINTRequest, now time.Time) {
	if caller == "" || request == nil || request.Kind != brevity.ELINTMud || request.Bearing != "" {
		return
	}
	p.mudLock.Lock()
	defer p.mudLock.Unlock()
	if p.pendingMuds == nil {
		p.pendingMuds = make(map[string]pendingMud)
	}
	p.pendingMuds[caller] = pendingMud{request: *request, at: now}
}

// ParseMudFollowUp reads a bare bearing ("1 8 0", "one eight zero", "mud 180") from a caller
// whose mud call had none, within MudFollowUpWindow. Returns nil otherwise.
func (p *Parser) ParseMudFollowUp(tx string, caller string, now time.Time) *brevity.ELINTRequest {
	if caller == "" || len(tx) > maxInputLength {
		return nil
	}
	p.mudLock.Lock()
	defer p.mudLock.Unlock()
	pending, ok := p.pendingMuds[caller]
	if !ok {
		return nil
	}
	if now.Sub(pending.at) > MudFollowUpWindow {
		delete(p.pendingMuds, caller)
		return nil
	}
	fields := strings.Fields(p.normalizeText(tx))
	if len(fields) > 0 && (fields[0] == "mud" || mudHomophones[fields[0]]) {
		fields = fields[1:]
	}
	bearing, end, ok := mudBearingAt(fields)
	if !ok || end != len(fields) {
		return nil
	}
	delete(p.pendingMuds, caller)
	request := pending.request
	request.Bearing = bearing
	return &request
}

func isEmitterWord(w string) bool {
	return isSimilar(w, "emitters") || isSimilar(w, "emitter") || isSimilar(w, "nearest")
}

// findELINTRequest finds the request in a call: its kind, where it starts and where it ends.
func findELINTRequest(fields []string) (kind brevity.ELINTRequestKind, start int, end int) {
	for i, w := range fields {
		next := ""
		if i+1 < len(fields) {
			next = fields[i+1]
		}
		switch {
		case w == wordSay && next != "" && isSimilar(next, "again"):
			return brevity.ELINTSayAgain, i, i + 2
		case w == "sayagain":
			return brevity.ELINTSayAgain, i, i + 1
		case w == "check" && next == "in", w == "checkin":
			end := i + 1
			if w == "check" {
				end++
			}
			return brevity.ELINTCheckIn, i, end
		case w == "a" && (isSimilar(next, "meter") || isSimilar(next, "meters")):
			return brevity.ELINTEmitters, i, i + 2
		case len(w) >= 5 && isEmitterWord(w):
			return brevity.ELINTEmitters, i, i + 1
		case len(w) >= 5 && (isSimilar(w, "status") || isSimilar(w, "target")):
			end := i + 1
			if end < len(fields) && len(fields[end]) >= 5 && (isSimilar(fields[end], "status") || isSimilar(fields[end], "target")) {
				end++
			}
			return brevity.ELINTTarget, i, end
		case w == "mud" || w == "dirt":
			return brevity.ELINTMud, i, i + 1
		case mudHomophones[w]:
			if _, ok := parseMudBearing(fields[i+1:]); ok {
				return brevity.ELINTMud, i, i + 1
			}
		}
	}
	return "", len(fields), len(fields)
}

// elintCallsign reads the pilot callsign from the words before the request, dropping lead-ins
// and filler, a callsign said twice ("Ford 9, Ford 9"), and words after its digits ("two ship").
func elintCallsign(fields []string) string {
	words := make([]string, 0, len(fields))
	for _, w := range fields {
		if !elintLeadIns[w] {
			words = append(words, w)
		}
	}
	if len(words) > 1 {
		if j := slices.Index(words[1:], words[0]); j >= 0 {
			words = words[:j+1]
		}
	}
	callsign, ok := callsigns.ParsePilotCallsign(leadingCallsign(numwords.ParseString(strings.Join(words, " "))))
	if !ok {
		return ""
	}
	return callsign
}

// ParseELINT reads a call to a mission ELINT speaker: "Vacuum, Ford 9, say emitters". onFrequencyOf
// is the speaker whose own frequency the call was heard on, or empty: there a call that names no
// one, or ANYFACE, is for that speaker, unless it names the GCI or a package controller. Returns nil
// when the call is not for a speaker.
func (p *Parser) ParseELINT(tx string, onFrequencyOf string) *brevity.ELINTRequest {
	speakers := p.speakerList()
	if len(speakers) == 0 || tx == "" || len(tx) > maxInputLength {
		return nil
	}
	tx = p.normalizeText(tx)
	fields := strings.Fields(tx)
	if len(fields) == 0 {
		return nil
	}
	speaker, rest, named := matchSpeaker(fields, speakers)
	if !named {
		own := strings.ToLower(strings.ReplaceAll(onFrequencyOf, " ", ""))
		if own == "" {
			return nil
		}
		anyface := isSimilar(fields[0], Anyface)
		if !anyface {
			lead := fields[:min(len(fields), maxSpeakerWords)]
			if _, _, _, ok := p.findControllerCallsign(lead); ok {
				return nil
			}
			if _, _, _, ok := p.findControllerAfterCallsign(fields); ok {
				return nil
			}
		}
		speaker, rest = own, fields
		if anyface {
			rest = fields[1:]
		}
	}
	kind, start, end := findELINTRequest(rest)
	if kind == "" {
		if !named {
			return nil
		}
		kind = brevity.ELINTUnknown
	}
	request := &brevity.ELINTRequest{
		Callsign: elintCallsign(rest[:start]),
		Speaker:  speaker,
		Kind:     kind,
	}
	if kind == brevity.ELINTMud {
		request.Bearing, _ = parseMudBearing(rest[end:])
	}
	return request
}
