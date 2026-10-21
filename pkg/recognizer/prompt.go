package recognizer

import (
	"sync"

	"github.com/dharmab/skyeye/pkg/recognizer/tuning"
	"github.com/rs/zerolog/log"
)

// promptState is a recognizer's initial prompt, rebuilt when the mission's vocabulary changes.
// See https://platform.openai.com/docs/guides/speech-to-text#prompting
type promptState struct {
	lock      sync.Mutex
	callsign  string
	locations []string
	vocab     tuning.Vocabulary
	prompt    string
}

func newPromptState(callsign string, locations []string) *promptState {
	return &promptState{
		callsign:  callsign,
		locations: locations,
		prompt:    tuning.Prompt(callsign, locations, tuning.Vocabulary{}),
	}
}

// current is the prompt, and whether it carries the mission's vocabulary.
func (p *promptState) current() (string, bool) {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.prompt, !p.vocab.IsZero()
}

// SetVocabulary implements [VocabularySetter.SetVocabulary].
func (p *promptState) SetVocabulary(vocab tuning.Vocabulary) {
	prompt := tuning.Prompt(p.callsign, p.locations, vocab)
	p.lock.Lock()
	changed := prompt != p.prompt
	p.vocab = vocab
	p.prompt = prompt
	p.lock.Unlock()
	if changed {
		log.Info().Int("chars", len(prompt)).Str("prompt", prompt).Msg("speech recognition prompt updated from the mission")
	}
}
