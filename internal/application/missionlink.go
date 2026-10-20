package application

import (
	"context"
	"time"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/dharmab/skyeye/pkg/composer"
	"github.com/dharmab/skyeye/pkg/missionlink"
	"github.com/dharmab/skyeye/pkg/simpleradio"
	"github.com/dharmab/skyeye/pkg/traces"
	"github.com/lithammer/shortuuid/v3"
	"github.com/rs/zerolog/log"
)

// frequencyKey carries the listen frequency a request was heard on, so its answer goes out on that
// frequency alone. Requests heard on the configured frequencies carry none.
type frequencyKey struct{}

func withFrequency(ctx context.Context, frequency *simpleradio.RadioFrequency) context.Context {
	if frequency == nil {
		return ctx
	}
	return context.WithValue(ctx, frequencyKey{}, *frequency)
}

func frequencyFrom(ctx context.Context) (simpleradio.RadioFrequency, bool) {
	frequency, ok := ctx.Value(frequencyKey{}).(simpleradio.RadioFrequency)
	return frequency, ok
}

// runMissionLink applies the mission's packages and speaks its lines.
func (a *Application) runMissionLink(ctx context.Context, out chan<- Message[composer.NaturalLanguageResponse]) {
	log.Info().Stringer("address", a.missionLink.Addr()).Msg("running mission link")
	a.missionLink.Run(ctx, missionlink.Handlers{
		OnPackages: func(packages missionlink.Packages) {
			a.parser.SetPackageControllers(packages.Controllers)
			frequencies := make([]simpleradio.RadioFrequency, 0, len(packages.Packages))
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
		},
		OnSay: func(say missionlink.Say) {
			frequency, err := simpleradio.ParseRadioFrequency(say.Frequency)
			if err != nil {
				log.Warn().Err(err).Str("frequency", say.Frequency).Msg("ignoring line with unreadable frequency")
				return
			}
			rCtx := traces.WithTraceID(traces.NewRequestContext(), shortuuid.New())
			rCtx = withFrequency(traces.WithRequestText(rCtx, say.Text), frequency)
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
	if frequency, ok := frequencyFrom(ctx); ok {
		message.Frequency = frequency.String()
	}
	if err := a.missionLink.Send(message); err != nil {
		log.Warn().Err(err).Msg("failed to pass package request to the mission")
	}
}
