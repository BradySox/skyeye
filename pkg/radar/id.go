package radar

import (
	"github.com/dharmab/skyeye/pkg/coalitions"
	"github.com/dharmab/skyeye/pkg/trackfiles"
)

// FindCallsign returns the trackfile on the given coalition that mosty closely matches the given callsign,
// or nil if no closely matching trackfile was found.
// The first return value is the callsign of the trackfile, and the second is the trackfile itself.
// The returned callsign may differ from the input callsign!
func (r *Radar) FindCallsign(callsign string, coalition coalitions.Coalition) (string, *trackfiles.Trackfile) {
	foundCallsign, tf, ok := r.contacts.getByCallsignAndCoalititon(callsign, coalition)
	if !ok {
		return callsign, nil
	}
	return foundCallsign, tf
}

// SetCallsignAliases lets FindCallsign find a player by the flight callsign they fly, such as "Ford
// 9-1", when their in-game name is something else, such as "Flash". Keys are callsigns, values the
// in-game names. Each call replaces the last.
func (r *Radar) SetCallsignAliases(aliases map[string]string) {
	r.contacts.setAliases(aliases)
}

// SpokenCallsign returns the flight callsign the GCI calls the trackfile named name by, such as
// "ford 9 1" for the player "Flash", when the mission says which seat that player flies. The
// second return value is false if it did not.
func (r *Radar) SpokenCallsign(name string) (string, bool) {
	return r.contacts.spokenCallsign(name)
}

// FindUnit returns the trackfile for the given unit ID, or nil if no trackfile was found.
func (r *Radar) FindUnit(id uint64) *trackfiles.Trackfile {
	trackfile, ok := r.contacts.getByID(id)
	if !ok {
		return nil
	}
	return trackfile
}

// FindByCoalition returns all trackfiles on the given coalition.
func (r *Radar) FindByCoalition(coalition coalitions.Coalition) []*trackfiles.Trackfile {
	contacts := []*trackfiles.Trackfile{}
	for trackfile := range r.contacts.values() {
		if trackfile.Contact.Coalition == coalition {
			contacts = append(contacts, trackfile)
		}
	}
	return contacts
}
