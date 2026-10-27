package application

import (
	"context"
	"time"

	"github.com/dharmab/skyeye/pkg/parser"
	"github.com/dharmab/skyeye/pkg/traces"
	"github.com/rs/zerolog/log"
)

// parse converts incoming brevity from text format to internal representations.
func (a *Application) parse(ctx context.Context, in <-chan Message[string], out chan<- Message[any]) {
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("stopping text parsing due to context cancellation")
			return
		case message := <-in:
			a.parseText(message.Context, message.Data, out)
		}
	}
}

// mudCaller names the radio and frequency a call came from, so a bare bearing after "say mud
// bearing" is matched to its own caller. Empty when either is unknown.
func mudCaller(ctx context.Context) string {
	name := traces.GetClientName(ctx)
	frequency, ok := heardFrequencyFrom(ctx)
	if name == "" || !ok {
		return ""
	}
	return name + "|" + frequency.String()
}

// parseText parses a single transcribed transmission, publishing any successfully parsed requests to the output channel.
func (a *Application) parseText(ctx context.Context, text string, out chan<- Message[any]) {
	logger := log.Logger
	if a.enableTranscriptionLogging {
		logger = logger.With().Str("text", text).Logger()
	}
	logger.Info().Msg("parsing text")
	var request any
	atc, loose := false, false
	if frequency, ok := heardFrequencyFrom(ctx); ok {
		atc, loose = a.isATCFrequency(frequency)
	}
	if atc {
		// An airfield frequency carries ATC calls only: the GCI never answers there.
		if call := parser.ParseATC(text, loose); call != nil {
			request = call
		}
	} else if call := a.parser.ParseMudFollowUp(text, mudCaller(ctx), time.Now()); call != nil {
		// A bare bearing answering Vacuum's "say mud bearing".
		request = call
	} else if call := a.parser.ParseELINT(text, a.elintSpeakerOn(ctx)); call != nil {
		// A call to Vacuum is the mission's to answer, never the GCI's.
		a.parser.AwaitMudBearing(mudCaller(ctx), call, time.Now())
		request = call
	} else {
		request = a.parser.Parse(text)
		if request == nil && a.heardOnLooseStrike(ctx) {
			if looseCall := a.parser.ParseLoosePackageCall(text); looseCall != nil {
				request = looseCall
			}
		}
	}
	ctx = traces.WithParsedAt(ctx, time.Now())
	if request != nil {
		ctx = traces.WithRequest(ctx, request)
		logger.Info().Any("request", request).Msg("parsed text")
		out <- AsMessage(ctx, request)
	} else {
		logger.Info().Msg("unable to parse text, could be silence, chatter, missing GCI callsign")
		a.trace(ctx)
	}
}
