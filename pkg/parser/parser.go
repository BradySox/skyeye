// Package parser converts brevity requests from natural language into structured forms.
package parser

import (
	"slices"
	"strings"
	"sync"

	"github.com/dharmab/numwords"
	"github.com/dharmab/skyeye/internal/normalize"
	"github.com/dharmab/skyeye/internal/parser/token"
	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/dharmab/skyeye/pkg/callsigns"
	"github.com/rs/zerolog/log"
)

func init() {
	numwords.IncludeFractions(false)
	numwords.IncludeIndefiniteArticle(false)
	numwords.IncludeSecond(false)
}

const (
	// maxInputLength is the maximum length of input text to process.
	// Prevents processing extremely long inputs that may indicate garbage data.
	maxInputLength = 1000
)

// Parser converts brevity requests from natural language into structured forms.
type Parser struct {
	controllerCallsign string
	enableTextLogging  bool
	// vectorLocations is the configured locations plus the tanker alias,
	// precomputed so parseVector does not mutate a shared slice.
	vectorLocations []string
	// packageControllers are extra callsigns, such as an AWACS running a strike package, whose
	// calls are passed to the mission as PackageRequests. Set at runtime by the mission link.
	packageControllers     []string
	packageControllersLock sync.RWMutex
	// speakers are the mission's ELINT speakers, such as Vacuum, lowercased without spaces. Their
	// calls are parsed by ParseELINT, never as GCI or package calls. Guarded by
	// packageControllersLock.
	speakers []string
	// pendingMuds are mud calls Vacuum asked the bearing of, by caller. Guarded by mudLock.
	pendingMuds map[string]pendingMud
	mudLock     sync.Mutex
}

// SetPackageControllers replaces the package controller callsigns the parser listens for.
func (p *Parser) SetPackageControllers(controllers []string) {
	normalized := make([]string, 0, len(controllers))
	for _, callsign := range controllers {
		if c := strings.ToLower(strings.ReplaceAll(callsign, " ", "")); c != "" {
			normalized = append(normalized, c)
		}
	}
	p.packageControllersLock.Lock()
	defer p.packageControllersLock.Unlock()
	p.packageControllers = normalized
}

// SetCallsign replaces the GCI callsign, for a mission that names the AWACS itself.
func (p *Parser) SetCallsign(callsign string) {
	p.packageControllersLock.Lock()
	defer p.packageControllersLock.Unlock()
	p.controllerCallsign = strings.ReplaceAll(callsign, " ", "")
}

func (p *Parser) callsign() string {
	p.packageControllersLock.RLock()
	defer p.packageControllersLock.RUnlock()
	return p.controllerCallsign
}

// wakePhrases is the GCI callsign, ANYFACE, then any package controller callsigns.
func (p *Parser) wakePhrases() []string {
	p.packageControllersLock.RLock()
	defer p.packageControllersLock.RUnlock()
	phrases := make([]string, 0, 2+len(p.packageControllers))
	phrases = append(phrases, p.controllerCallsign, Anyface)
	return append(phrases, p.packageControllers...)
}

// isPackageController is true if the wake phrase is a package controller callsign.
func (p *Parser) isPackageController(phrase string) bool {
	p.packageControllersLock.RLock()
	defer p.packageControllersLock.RUnlock()
	return slices.Contains(p.packageControllers, phrase)
}

// New creates a new parser.
func New(callsign string, locations []string, enableTextLogging bool) *Parser {
	vectorLocations := make([]string, 0, len(locations)+1)
	vectorLocations = append(vectorLocations, locations...)
	vectorLocations = append(vectorLocations, brevity.LocationTanker)
	return &Parser{
		controllerCallsign: strings.ReplaceAll(callsign, " ", ""),
		enableTextLogging:  enableTextLogging,
		vectorLocations:    vectorLocations,
	}
}

// normalizeText lowercases and cleans a transcription the way Parse reads it.
func (p *Parser) normalizeText(tx string) string {
	tx = normalize.Normalize(tx)
	for _, replacement := range replacements {
		tx = strings.ReplaceAll(tx, replacement.Original, replacement.Normal)
	}
	if tx == "" {
		return ""
	}
	return p.uncrushCallsign(tx)
}

// ParseLoosePackageCall reads a package call that names only the pilot, such as "Colt 1 1, ready to
// push", for the first package controller. Only for text heard on the strike frequency. Returns nil
// when it is not a package call.
func (p *Parser) ParseLoosePackageCall(tx string) *brevity.PackageRequest {
	p.packageControllersLock.RLock()
	controller := ""
	if len(p.packageControllers) > 0 {
		controller = p.packageControllers[0]
	}
	p.packageControllersLock.RUnlock()
	if controller == "" || len(tx) > maxInputLength {
		return nil
	}
	tx = p.normalizeText(tx)
	fields := strings.Fields(tx)
	var before []string
	request := (*brevity.PackageRequest)(nil)
	if word, idx, ok := findRequestWord(fields); ok && word == push {
		before = trimPushPreamble(fields[:idx])
		request = parsePush("", controller, fields[idx+1:])
	} else if i := strings.Index(tx, checkIn); i > 0 {
		before = strings.Fields(tx[:i])
		request = &brevity.PackageRequest{Controller: controller, Kind: brevity.PackageCheckIn}
	} else {
		return nil
	}
	pilot, ok := callsigns.ParsePilotCallsign(leadingCallsign(numwords.ParseString(strings.Join(before, " "))))
	if !ok {
		return nil
	}
	request.Callsign = pilot
	return request
}

// Anyface is a brevity codeword that can be used in place of a GCI callsign.
const Anyface string = "anyface"

const (
	alphaCheck string = "alpha"
	bogeyDope  string = "bogey"
	checkIn    string = "check in"
	declare    string = "declare"
	picture    string = "picture"
	radioCheck string = "radio"
	shopping   string = "shopping"
	snaplock   string = "snaplock"
	spiked     string = "spiked"
	strobe     string = "strobe"
	tripwire   string = "tripwire"
	vector     string = "vector"
	push       string = "push"
	pushTime   string = "time"
)

var requestWords = []string{radioCheck, alphaCheck, bogeyDope, declare, picture, spiked, strobe, snaplock, tripwire, shopping, vector, push}

// findControllerCallsign searches for the GCI callsign, ANYFACE or a package controller callsign in
// the given fields. Returns the heard callsign, the wake phrase it matched, the remaining text after
// it, and whether it was found.
func (p *Parser) findControllerCallsign(fields []string) (heard string, matched string, rest string, ok bool) {
	phrases := p.wakePhrases()
	for i := range fields {
		candidate := strings.Join(fields[:i+1], " ")
		for _, wakePhrase := range phrases {
			if isSimilar(strings.TrimSpace(candidate), strings.ToLower(wakePhrase)) {
				ok = true
				heard = candidate
				matched = strings.ToLower(wakePhrase)
				rest = strings.Join(fields[i+1:], " ")
				return
			}
		}
	}
	return "", "", "", false
}

// pushPreamble are words a pilot says between their callsign and PUSH: "ready to push", "say push
// time". They are dropped before the callsign is read, or "to" reads as a "2".
var pushPreamble = map[string]bool{"ready": true, "to": true, "say": true, "request": true, "for": true}

func trimPushPreamble(fields []string) []string {
	for len(fields) > 0 && pushPreamble[fields[len(fields)-1]] {
		fields = fields[:len(fields)-1]
	}
	return fields
}

// parsePush reads a PUSH request: "say push time" or "push time" asks for the time, anything else
// with "push" in it is a flight reporting ready to push.
func parsePush(pilotCallsign, controller string, requestArgs []string) *brevity.PackageRequest {
	kind := brevity.PackageReadyToPush
	for _, arg := range requestArgs {
		if isSimilar(pushTime, arg) {
			kind = brevity.PackagePushTime
			break
		}
	}
	return &brevity.PackageRequest{Callsign: pilotCallsign, Controller: controller, Kind: kind}
}

// handleNoRequestWord handles cases where we heard the GCI callsign and pilot callsign
// but couldn't identify a specific request word.
func handleNoRequestWord(tx, pilotCallsign string) any {
	if strings.Contains(tx, checkIn) {
		return &brevity.CheckInRequest{Callsign: pilotCallsign}
	}
	return &brevity.UnableToUnderstandRequest{Callsign: pilotCallsign}
}

// parseRequestWithArgs attempts to parse a request that requires additional arguments
// beyond the request word itself (e.g., BOGEY DOPE, DECLARE, SPIKED).
func (p *Parser) parseRequestWithArgs(requestWord, pilotCallsign string, requestArgs []string) any {
	stream := token.New(strings.Join(requestArgs, " "))

	switch requestWord {
	case bogeyDope:
		if request, ok := parseBogeyDope(pilotCallsign, stream); ok {
			return request
		}
	case declare:
		if request, ok := parseDeclare(pilotCallsign, stream); ok {
			return request
		}
	case spiked:
		if request, ok := parseSpiked(pilotCallsign, stream); ok {
			return request
		}
	case strobe:
		if request, ok := parseStrobe(pilotCallsign, stream); ok {
			return request
		}
	case snaplock:
		if request, ok := parseSnaplock(pilotCallsign, stream); ok {
			return request
		}
	case vector:
		if request, ok := parseVector(pilotCallsign, p.vectorLocations, stream); ok {
			return request
		}
	}

	return &brevity.UnableToUnderstandRequest{Callsign: pilotCallsign}
}

func findRequestWord(fields []string) (string, int, bool) {
	for i, field := range fields {
		field = strings.TrimPrefix(field, "request")
		for _, word := range requestWords {
			if isSimilar(word, field) {
				return word, i, true
			}
			// HACK: Also compare the first half of long fields separately.
			// Handles some cases of two words running together, e.g.
			// "bogeydope" instead of "bogey dope".
			if len(field) > halfFieldMinLength {
				halfField := field[:len(field)/2]
				if isSimilar(word, halfField) {
					return word, i, true
				}
			}
		}
	}
	return "", 0, false
}

// uncrushCallsign corrects a corner case where the GCI callsign and the
// following token have no space between them, e.g. "anyfaceeagle 1".
func (p *Parser) uncrushCallsign(s string) string {
	for _, callsign := range p.wakePhrases() {
		lc := strings.ToLower(callsign)
		if strings.HasPrefix(s, lc) {
			return lc + " " + s[len(lc):]
		}
	}
	return s
}

// Parse reads natural language text, checks if it starts with the GCI
// callsign, and attempts to parse a request from the text. Returns a
// brevity request, or nil if the text does not start with the GCI
// callsign.
func (p *Parser) Parse(tx string) any {
	if tx == "" {
		return nil
	}
	if len(tx) > maxInputLength {
		log.Warn().Int("length", len(tx)).Msg("unusually long input truncated")
		tx = tx[:maxInputLength]
	}

	logger := log.With().Str("gci", p.callsign()).Logger()
	if p.enableTextLogging {
		logger = logger.With().Str("text", tx).Logger()
	}
	logger.Debug().Msg("parsing text")
	tx = p.normalizeText(tx)
	if tx == "" {
		return nil
	}

	if p.enableTextLogging {
		logger = logger.With().Str("normalized", tx).Logger()
	}
	logger.Debug().Msg("normalized text")

	fields := fixSayHomophones(strings.Fields(tx))

	before := fields
	var requestArgs []string
	requestWord, idx, ok := findRequestWord(fields)
	if ok {
		logger = logger.With().Str("request", requestWord).Logger()
		logger.Debug().Int("position", idx).Msg("found request word")
		before, requestArgs = fields[:idx], fields[idx+1:]
		if requestWord == push {
			before = trimPushPreamble(before)
		}
	}

	heard, matched, rest, ok := p.findControllerCallsign(before)
	if !ok {
		heard, matched, rest, ok = p.findControllerAfterCallsign(before)
	}
	if !ok && requestWord != "" {
		heard, matched, rest, ok = p.findControllerAfterFlightName(before)
	}

	if !ok {
		logger.Trace().Msg("no GCI callsign found")
		return nil
	}
	event := logger.Debug().Str("heard", heard)
	if p.enableTextLogging {
		event = event.Str("rest", rest)
	}
	event.Msg("found GCI callsign")

	event = logger.Debug()
	if p.enableTextLogging {
		event = event.Str("rest", rest)
	}
	event.Msg("searching for pilot callsign in rest of text")

	rest = trimLeadIns(leadingCallsign(numwords.ParseString(rest)))
	pilotCallsign, ok := callsigns.ParsePilotCallsign(rest)
	if ok {
		logger = logger.With().Str("pilot", pilotCallsign).Logger()
		logger.Debug().Msg("found pilot callsign")
	} else {
		logger.Trace().Msg("no pilot callsign found")
	}

	if !ok {
		if requestWord != "" && requestWord == picture {
			return &brevity.PictureRequest{Callsign: ""}
		}
		if p.isPackageController(matched) && requestWord == "" {
			return parsePackageCallWithoutRequestWord(tx, "", matched)
		}
		return &brevity.UnableToUnderstandRequest{}
	}
	if requestWord == "" {
		logger.Trace().Msg("no request word found")
		if p.isPackageController(matched) {
			return parsePackageCallWithoutRequestWord(tx, pilotCallsign, matched)
		}
		return handleNoRequestWord(tx, pilotCallsign)
	}
	if requestWord == push {
		return parsePush(pilotCallsign, matched, requestArgs)
	}

	switch requestWord {
	case alphaCheck:
		return &brevity.AlphaCheckRequest{Callsign: pilotCallsign}
	case radioCheck:
		return &brevity.RadioCheckRequest{Callsign: pilotCallsign}
	case picture:
		return &brevity.PictureRequest{Callsign: pilotCallsign}
	case tripwire:
		return &brevity.TripwireRequest{Callsign: pilotCallsign}
	case shopping:
		return &brevity.ShoppingRequest{Callsign: pilotCallsign}
	}

	event = logger.Debug()
	if p.enableTextLogging {
		event = event.Strs("args", requestArgs)
	}
	event.Msg("parsing request arguments")

	return p.parseRequestWithArgs(requestWord, pilotCallsign, requestArgs)
}
