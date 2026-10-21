package recognizer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/dharmab/skyeye/pkg/pcm/rate"
	"github.com/dharmab/skyeye/pkg/simpleradio"
	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/rs/zerolog/log"
)

type whisperRecognizer struct {
	model whisper.Model
	recognizerOptions
	*promptState

	// lock serializes recognitions. Every context of a model shares the model's one whisper.cpp
	// state, so two recognitions at once would corrupt each other.
	lock sync.Mutex
	// wCtx is reused across transmissions; nil until the first one.
	wCtx whisper.Context
	// wPrompt is the prompt set on wCtx. The binding copies each prompt into C memory it never
	// frees, so it is set only when it changes.
	wPrompt string
}

var (
	_ Recognizer       = &whisperRecognizer{}
	_ VocabularySetter = &whisperRecognizer{}
	_ WarmUpper        = &whisperRecognizer{}
)

// NewWhisperRecognizer creates a new recognizer using OpenAI Whisper.
func NewWhisperRecognizer(model *whisper.Model, callsign string, opts ...Option) Recognizer {
	r := &whisperRecognizer{
		model: *model,
	}
	for _, opt := range opts {
		opt(&r.recognizerOptions)
	}
	r.promptState = newPromptState(callsign, r.recognizerOptions.locations)
	return r
}

// whisperContext is the reused whisper context with the current prompt. Call it with r.lock held.
func (r *whisperRecognizer) whisperContext(prompt string) (whisper.Context, error) {
	if r.wCtx == nil {
		wCtx, err := r.model.NewContext()
		if err != nil {
			return nil, fmt.Errorf("error creating whisper context: %w", err)
		}
		if r.threads > 0 {
			wCtx.SetThreads(r.threads)
		}
		if wCtx.IsMultilingual() {
			_ = wCtx.SetLanguage("en")
		}
		r.wCtx = wCtx
		r.wPrompt = ""
	}
	if prompt != r.wPrompt {
		r.wCtx.SetInitialPrompt(prompt)
		r.wPrompt = prompt
	}
	return r.wCtx, nil
}

// WarmUp implements [WarmUpper.WarmUp]. The first whisper.cpp run on a GPU builds its pipelines,
// which cost the first real transmission over ten seconds on Vulkan.
func (r *whisperRecognizer) WarmUp(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	silence := make([]float32, int(rate.Wideband.Hertz()))
	_, err := r.Recognize(ctx, silence, false)
	return time.Since(start), err
}

// Recognize implements [Recognizer.Recognize] using whisper.cpp.
func (r *whisperRecognizer) Recognize(ctx context.Context, sample []float32, enableTranscriptionLogging bool) (string, error) {
	maxSize := int(simpleradio.MaxTransmissionDuration.Seconds()) * int(rate.Wideband.Hertz())
	if len(sample) > maxSize {
		log.Warn().Int("length", len(sample)).Int("maxLength", maxSize).Msg("clamping sample to maximum size")
		sample = sample[:maxSize]
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	prompt, missionVocabulary := r.current()
	wCtx, err := r.whisperContext(prompt)
	if err != nil {
		return "", err
	}

	start := time.Now()
	err = wCtx.Process(
		sample,
		nil,
		func(segment whisper.Segment) {
			event := log.Debug()
			if enableTranscriptionLogging {
				event = event.Str("text", segment.Text)
			}
			event.Msg("processing segment")
		},
		nil,
	)
	log.Info().
		Stringer("audioLength", time.Duration(float64(len(sample))/rate.Wideband.Hertz()*float64(time.Second))).
		Stringer("recognitionTime", time.Since(start)).
		Uint("threads", r.threads).
		Int("promptChars", len(prompt)).
		Bool("missionVocabulary", missionVocabulary).
		Msg("whisper recognition timing")
	if err != nil {
		return "", fmt.Errorf("error processing sample: %w", err)
	}

	var textBuilder strings.Builder
	for {
		select {
		case <-ctx.Done():
			log.Warn().Msg("returning early from speech recognition due to context cancellation")
			return textBuilder.String(), nil
		default:
			segment, err := wCtx.NextSegment()
			if errors.Is(err, io.EOF) {
				return textBuilder.String(), nil
			}
			if err != nil {
				return textBuilder.String(), fmt.Errorf("error processing segment: %w", err)
			}
			if _, err := textBuilder.WriteString(segment.Text); err != nil {
				return textBuilder.String(), fmt.Errorf("error writing segment text: %w", err)
			}
		}
	}
}
