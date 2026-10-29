package application

import (
	"slices"

	"github.com/dharmab/skyeye/pkg/recognizer"
	"github.com/dharmab/skyeye/pkg/recognizer/tuning"
)

// missionVocabulary is what each mission link message has told SkyEye that pilots will say.
// Each message replaces only its own part.
type missionVocabulary struct {
	gciCallsign string
	// packageCallsigns are the package controllers and package names.
	packageCallsigns []string
	// atcCallsigns are the flights the mission runs ATC for.
	atcCallsigns []string
	fields       []string
	// speakers are the callsigns of the mission's other speakers, such as Vacuum.
	speakers []string
}

func (v missionVocabulary) merged() tuning.Vocabulary {
	return tuning.Vocabulary{
		Callsign:  v.gciCallsign,
		Callsigns: slices.Concat(v.packageCallsigns, v.atcCallsigns, v.speakers),
		Fields:    slices.Clone(v.fields),
	}
}

// updateVocabulary applies a change to the mission's vocabulary and passes the result to the
// recognizer's prompt.
func (a *Application) updateVocabulary(update func(*missionVocabulary)) {
	a.vocabularyLock.Lock()
	update(&a.vocabulary)
	vocab := a.vocabulary.merged()
	a.vocabularyLock.Unlock()
	if setter, ok := a.recognizer.(recognizer.VocabularySetter); ok {
		setter.SetVocabulary(vocab)
	}
}

// addSpeakerVocabulary adds a mission speaker's callsign to the vocabulary once.
// fieldNames are the airfields the mission runs ATC for.
func (a *Application) fieldNames() []string {
	a.vocabularyLock.Lock()
	defer a.vocabularyLock.Unlock()
	return slices.Clone(a.vocabulary.fields)
}

func (a *Application) addSpeakerVocabulary(callsign string) {
	if callsign == "" {
		return
	}
	a.vocabularyLock.Lock()
	known := slices.Contains(a.vocabulary.speakers, callsign)
	a.vocabularyLock.Unlock()
	if known {
		return
	}
	a.updateVocabulary(func(v *missionVocabulary) {
		if !slices.Contains(v.speakers, callsign) {
			v.speakers = append(v.speakers, callsign)
		}
	})
}
