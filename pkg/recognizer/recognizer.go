// Package recognizer recognizes text from speech.
package recognizer

import (
	"context"
	"time"

	"github.com/dharmab/skyeye/pkg/recognizer/tuning"
)

// Recognizer recognizes text from speech.
type Recognizer interface {
	// Recognize takes PCMF32LE audio data and returns any recognized text.
	Recognize(ctx context.Context, pcm []float32, enableTranscriptionLogging bool) (string, error)
}

// VocabularySetter is a Recognizer whose prompt follows what the mission tells SkyEye.
type VocabularySetter interface {
	// SetVocabulary replaces the mission's vocabulary in the recognizer's prompt.
	SetVocabulary(vocab tuning.Vocabulary)
}

// WarmUpper is a Recognizer that pays a first-use cost, such as GPU shader setup, which it can
// pay before the first real transmission.
type WarmUpper interface {
	// WarmUp runs one recognition on silence and returns how long it took.
	WarmUp(ctx context.Context) (time.Duration, error)
}
