package radar

import (
	"iter"
	"slices"
	"strings"
	"sync"

	"github.com/dharmab/skyeye/pkg/callsigns"
	"github.com/dharmab/skyeye/pkg/coalitions"
	"github.com/dharmab/skyeye/pkg/trackfiles"
	fuzz "github.com/hbollon/go-edlib"
	"github.com/rs/zerolog/log"
)

// CallsignSimilarityThreshold is the minimum Levenshtein similarity score
// required for fuzzy callsign matching (0.0-1.0).
const CallsignSimilarityThreshold = 0.63

// contactDatabase is a thread-safe trackfile contactDatabase.
type contactDatabase struct {
	lock        sync.RWMutex
	contacts    map[uint64]*trackfiles.Trackfile
	callsignIdx map[coalitions.Coalition]map[string]uint64
	// aliases maps a flight callsign a pilot says, such as "ford 9 1", to the name of the
	// trackfile flying it, such as the player name "Flash". Set by the mission.
	aliases map[string]string
	// spoken maps a trackfile's name, such as "Flash", to the flight callsign the GCI calls it
	// by, such as "ford 9 1". Set by the mission with the aliases.
	spoken map[string]string
}

func newContactDatabase() *contactDatabase {
	db := &contactDatabase{}
	db.reset()
	return db
}

// getByCallsignAndCoalititon returns the trackfile with the lowest edit distance to the given callsign, or nil if no closely named trackfile was found.
// The second return value is true if a trackfile was found, and false otherwise.
// The callsign in the trackfile may differ from the input callsign!
func (db *contactDatabase) getByCallsignAndCoalititon(callsign string, coalition coalitions.Coalition) (string, *trackfiles.Trackfile, bool) {
	logger := log.With().Str("callsign", callsign).Str("coalition", coalition.String()).Logger()
	db.lock.RLock()
	defer db.lock.RUnlock()

	// Normalize at lookup time to keep raw names as index keys (benchmarked ~37-47µs with 40 contacts on Apple M4).
	normalized := make(map[string]uint64, len(db.callsignIdx[coalition]))
	for rawName, id := range db.callsignIdx[coalition] {
		parsed, ok := callsigns.ParsePilotCallsign(rawName)
		if !ok {
			parsed = rawName
		}
		normalized[parsed] = id
	}

	foundCallsign := ""
	id, ok := normalized[callsign]
	if !ok {
		if name, isAlias := db.aliases[callsign]; isAlias {
			id, ok = db.callsignIdx[coalition][name]
		}
	}
	if ok {
		foundCallsign = callsign
	} else {
		keys := make([]string, 0, len(normalized))
		for k := range normalized {
			keys = append(keys, k)
		}
		logger.Info().Msg("callsign not found in index, attempting fuzzy search")
		var err error
		foundCallsign, err = fuzz.FuzzySearchThreshold(callsign, keys, CallsignSimilarityThreshold, fuzz.Levenshtein)
		if foundCallsign == "" || err != nil {
			logger.Warn().Err(err).Msg("callsign not found in index")
			return "", nil, false
		}
		logger.Info().Str("foundCallsign", foundCallsign).Msg("similar callsign found in index")
		id = normalized[foundCallsign]
	}
	contact, ok := db.contacts[id]
	if !ok {
		return "", nil, false
	}
	return foundCallsign, contact, true
}

// setAliases replaces the flight callsigns the database also finds trackfiles by: each key is a
// callsign as a pilot says it ("Ford 9-1"), each value the trackfile's name ("Flash").
func (db *contactDatabase) setAliases(aliases map[string]string) {
	normalized := make(map[string]string, len(aliases))
	spoken := make(map[string]string, len(aliases))
	for callsign, name := range aliases {
		parsed, ok := callsigns.ParsePilotCallsign(callsign)
		if !ok || name == "" {
			continue
		}
		normalized[parsed] = name
		if current, ok := spoken[name]; !ok || isPreferredSpokenCallsign(parsed, current) {
			spoken[name] = parsed
		}
	}
	db.lock.Lock()
	defer db.lock.Unlock()
	db.aliases = normalized
	db.spoken = spoken
}

// isPreferredSpokenCallsign is true if candidate names a player better than current: "ford 9" is
// the lead of "ford 9 1", so the seat, with more words, wins. A tie goes to the first in order.
func isPreferredSpokenCallsign(candidate, current string) bool {
	if a, b := len(strings.Fields(candidate)), len(strings.Fields(current)); a != b {
		return a > b
	}
	return candidate < current
}

// spokenCallsign returns the flight callsign the mission says the trackfile with the given name
// flies, such as "ford 9 1" for "Flash". The second return value is false if the mission did not
// say.
func (db *contactDatabase) spokenCallsign(name string) (string, bool) {
	db.lock.RLock()
	defer db.lock.RUnlock()
	callsign, ok := db.spoken[name]
	return callsign, ok
}

// getByID returns the trackfile for the given unit ID, or nil if no trackfile was found.
// The second return value is true if a trackfile was found, and false otherwise.
func (db *contactDatabase) getByID(id uint64) (*trackfiles.Trackfile, bool) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	contact, ok := db.contacts[id]
	return contact, ok
}

// set updates the trackfile for the given trackfile's unit ID, or inserts a new trackfile if no trackfile was found.
func (db *contactDatabase) set(trackfile *trackfiles.Trackfile) {
	db.lock.Lock()
	defer db.lock.Unlock()

	db.callsignIdx[trackfile.Contact.Coalition][trackfile.Contact.Name] = trackfile.Contact.ID
	db.contacts[trackfile.Contact.ID] = trackfile
}

// delete removes the trackfile for the given unit ID.
func (db *contactDatabase) delete(id uint64) bool {
	db.lock.Lock()
	defer db.lock.Unlock()

	contact, ok := db.contacts[id]
	if ok {
		delete(db.callsignIdx[contact.Contact.Coalition], contact.Contact.Name)
	}
	delete(db.contacts, id)

	return ok
}

// reset removes all trackfiles from the database.
func (db *contactDatabase) reset() {
	db.lock.Lock()
	defer db.lock.Unlock()
	db.contacts = make(map[uint64]*trackfiles.Trackfile)
	db.callsignIdx = make(map[coalitions.Coalition]map[string]uint64)
	for _, c := range []coalitions.Coalition{coalitions.Blue, coalitions.Red, coalitions.Neutrals} {
		db.callsignIdx[c] = make(map[string]uint64)
	}
}

// values iterates over all trackfiles in the database.
func (db *contactDatabase) values() iter.Seq[*trackfiles.Trackfile] {
	db.lock.RLock()
	defer db.lock.RUnlock()

	contacts := make([]*trackfiles.Trackfile, 0, len(db.contacts))
	for _, contact := range db.contacts {
		contacts = append(contacts, contact)
	}
	return slices.Values(contacts)
}
