package application

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/dharmab/skyeye/pkg/composer"
	"github.com/dharmab/skyeye/pkg/missionlink"
	"github.com/dharmab/skyeye/pkg/simpleradio"
	"github.com/dharmab/skyeye/pkg/synthesizer/speakers"
	"github.com/dharmab/skyeye/pkg/synthesizer/voices"
	"github.com/dharmab/skyeye/pkg/traces"
	"github.com/lithammer/shortuuid/v3"
	"github.com/rs/zerolog/log"
)

type (
	// targetKey carries the one frequency an answer goes out on: the listen frequency its
	// request was heard on, or the frequency a mission line names.
	targetKey struct{}
	// heardKey carries the frequency a request was heard on, configured or listen.
	heardKey struct{}
	// importantKey marks a call that also goes out on the strike frequency.
	importantKey struct{}
	// voiceKey carries the voice a mission line asks for.
	voiceKey struct{}
)

func withFrequency(ctx context.Context, frequency *simpleradio.RadioFrequency) context.Context {
	if frequency == nil {
		return ctx
	}
	return context.WithValue(ctx, targetKey{}, *frequency)
}

func frequencyFrom(ctx context.Context) (simpleradio.RadioFrequency, bool) {
	frequency, ok := ctx.Value(targetKey{}).(simpleradio.RadioFrequency)
	return frequency, ok
}

func withHeardFrequency(ctx context.Context, frequency *simpleradio.RadioFrequency) context.Context {
	if frequency == nil {
		return ctx
	}
	return context.WithValue(ctx, heardKey{}, *frequency)
}

func heardFrequencyFrom(ctx context.Context) (simpleradio.RadioFrequency, bool) {
	frequency, ok := ctx.Value(heardKey{}).(simpleradio.RadioFrequency)
	return frequency, ok
}

func withImportant(ctx context.Context) context.Context {
	return context.WithValue(ctx, importantKey{}, true)
}

func isImportant(ctx context.Context) bool {
	important, _ := ctx.Value(importantKey{}).(bool)
	return important
}

func withVoice(ctx context.Context, voice string) context.Context {
	return context.WithValue(ctx, voiceKey{}, voice)
}

func voiceFrom(ctx context.Context) (string, bool) {
	voice, ok := ctx.Value(voiceKey{}).(string)
	return voice, ok
}

// isImportantCall is true for calls a pilot on the strike frequency must hear. Everything else
// SkyEye broadcasts stays off strike, so the GCI does not step on the package traffic.
func isImportantCall(call any) bool {
	switch call.(type) {
	case brevity.ThreatCall, brevity.MergedCall:
		return true
	}
	return false
}

// Voice names: "feminine" and "masculine" are the built-in voices; anything else is a Piper voice
// folder under the voices directory, optionally "#speaker" for one speaker of a multi-speaker
// model, such as "en_US-l2arctic-medium#ABA".
const (
	feminineVoiceName  = "feminine"
	masculineVoiceName = "masculine"
)

func builtinVoiceName(voice voices.Voice) string {
	if voice == voices.MasculineVoice {
		return masculineVoiceName
	}
	return feminineVoiceName
}

func parseVoice(name string) (string, bool) {
	name = strings.TrimSpace(name)
	return name, name != ""
}

func (a *Application) newSpeaker(voice string) (speakers.Speaker, error) {
	switch voice {
	case feminineVoiceName:
		return speakers.NewPiperSpeaker(voices.FeminineVoice, a.voiceSpeed, a.voicePause)
	case masculineVoiceName:
		return speakers.NewPiperSpeaker(voices.MasculineVoice, a.voiceSpeed, a.voicePause)
	}
	return speakers.NewDiskSpeaker(a.voicesDir, voice, a.voiceSpeed, a.voicePause)
}

// speakerFor is the speaker for a call: the voice its context asks for, else SkyEye's own voice.
// Piper speakers for other voices are built on first use. macOS has the one system speaker.
func (a *Application) speakerFor(ctx context.Context) (speakers.Speaker, error) {
	a.speakersLock.Lock()
	defer a.speakersLock.Unlock()
	if runtime.GOOS == "darwin" {
		return a.speaker, nil
	}
	voice, ok := voiceFrom(ctx)
	if !ok {
		voice = a.gciVoice
	}
	if speaker, ok := a.voiceSpeakers[voice]; ok {
		return speaker, nil
	}
	speaker, err := a.newSpeaker(voice)
	if err != nil {
		// A missing or broken voice falls back to SkyEye's own, so the call is still heard.
		log.Warn().Err(err).Str("voice", voice).Msg("voice unavailable, using the default voice")
		a.voiceSpeakers[voice] = a.speaker
		return a.speaker, nil
	}
	a.voiceSpeakers[voice] = speaker
	return speaker, nil
}

// currentComposer is a copy of the composer under the callsign lock.
func (a *Application) currentComposer() composer.Composer {
	a.speakersLock.Lock()
	defer a.speakersLock.Unlock()
	return a.composer
}

// strikeFrequency is the mission's strike frequency, if a mission has sent one.
func (a *Application) strikeFrequency() (simpleradio.RadioFrequency, bool) {
	a.speakersLock.Lock()
	defer a.speakersLock.Unlock()
	if a.strike == nil {
		return simpleradio.RadioFrequency{}, false
	}
	return *a.strike, true
}

func (a *Application) applyPackages(packages missionlink.Packages) {
	a.parser.SetPackageControllers(packages.Controllers)
	frequencies := make([]simpleradio.RadioFrequency, 0, len(packages.Packages)+1)
	if packages.Strike != "" {
		strike, err := simpleradio.ParseRadioFrequency(packages.Strike)
		if err != nil {
			log.Warn().Err(err).Str("frequency", packages.Strike).Msg("ignoring unreadable strike frequency")
		} else {
			frequencies = append(frequencies, *strike)
			a.speakersLock.Lock()
			a.strike = strike
			a.speakersLock.Unlock()
		}
	}
	if packages.GCICallsign != "" {
		a.parser.SetCallsign(packages.GCICallsign)
		a.speakersLock.Lock()
		a.composer.Callsign = packages.GCICallsign
		a.speakersLock.Unlock()
		log.Info().Str("callsign", packages.GCICallsign).Msg("mission set the GCI callsign")
	}
	if voice, ok := parseVoice(packages.GCIVoice); ok {
		a.speakersLock.Lock()
		a.gciVoice = voice
		a.speakersLock.Unlock()
	}
	for _, pkg := range packages.Packages {
		frequency, err := simpleradio.ParseRadioFrequency(pkg.Frequency)
		if err != nil {
			log.Warn().Err(err).Str("package", pkg.Name).Msg("ignoring package with unreadable frequency")
			continue
		}
		frequencies = append(frequencies, *frequency)
	}
	if err := a.srsClient.AddListenRadios(frequencies); err != nil {
		log.Error().Err(err).Msg("failed to tune package frequencies")
	}
}

// runMissionLink applies the mission's packages and speaks its lines.
func (a *Application) runMissionLink(ctx context.Context, out chan<- Message[composer.NaturalLanguageResponse]) {
	log.Info().Stringer("address", a.missionLink.Addr()).Msg("running mission link")
	a.missionLink.Run(ctx, missionlink.Handlers{
		OnPackages: a.applyPackages,
		OnSay: func(say missionlink.Say) {
			frequency, err := simpleradio.ParseRadioFrequency(say.Frequency)
			if err != nil {
				log.Warn().Err(err).Str("frequency", say.Frequency).Msg("ignoring line with unreadable frequency")
				return
			}
			rCtx := traces.WithTraceID(traces.NewRequestContext(), shortuuid.New())
			rCtx = withFrequency(traces.WithRequestText(rCtx, say.Text), frequency)
			if voice, ok := parseVoice(say.Voice); ok {
				rCtx = withVoice(rCtx, voice)
			}
			select {
			case out <- AsMessage(rCtx, composer.NaturalLanguageResponse{Speech: say.Text, Subtitle: say.Text}):
			case <-ctx.Done():
			case <-time.After(30 * time.Second):
				log.Warn().Msg("dropped mission line: speech queue is full")
			}
		},
	})
}

// handlePackageRequest passes a package request to the mission, which answers with a say message.
func (a *Application) handlePackageRequest(ctx context.Context, request *brevity.PackageRequest) {
	if a.missionLink == nil {
		a.controller.HandleUnableToUnderstand(ctx, &brevity.UnableToUnderstandRequest{Callsign: request.Callsign})
		return
	}
	message := missionlink.Request{
		Kind:       string(request.Kind),
		Callsign:   request.Callsign,
		Controller: request.Controller,
	}
	if frequency, ok := heardFrequencyFrom(ctx); ok {
		message.Frequency = frequency.String()
	}
	if err := a.missionLink.Send(message); err != nil {
		log.Warn().Err(err).Msg("failed to pass package request to the mission")
	}
}
