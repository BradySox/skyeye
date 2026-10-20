package application

import (
	"context"
	"errors"
	"time"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/dharmab/skyeye/pkg/composer"
	"github.com/dharmab/skyeye/pkg/controller"
	"github.com/dharmab/skyeye/pkg/traces"
	"github.com/rs/zerolog/log"
)

// compose converts outgoing brevity from internal representations to text format.
func (a *Application) compose(ctx context.Context, in <-chan controller.Call, out chan<- Message[composer.NaturalLanguageResponse]) {
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("stopping brevity composition due to context cancellation")
			return
		case call := <-in:
			a.composeCall(call.Context, call.Call, out)
		}
	}
}

// composeCall handles a single call, publishing the composition to the output channel.
func (a *Application) composeCall(ctx context.Context, call any, out chan<- Message[composer.NaturalLanguageResponse]) {
	ctx = traces.WithHandledAt(ctx, time.Now())
	logger := log.With().Type("type", call).Any("params", call).Logger()
	logger.Info().Msg("composing brevity call")
	var response composer.NaturalLanguageResponse
	comp := a.currentComposer()
	switch c := call.(type) {
	case brevity.AlphaCheckResponse:
		response = comp.ComposeAlphaCheckResponse(c)
	case brevity.BogeyDopeResponse:
		response = comp.ComposeBogeyDopeResponse(c)
	case brevity.CheckInResponse:
		response = comp.ComposeCheckInResponse(c)
	case brevity.DeclareResponse:
		response = comp.ComposeDeclareResponse(c)
	case brevity.FadedCall:
		response = comp.ComposeFadedCall(c)
	case brevity.NegativeRadarContactResponse:
		response = comp.ComposeNegativeRadarContactResponse(c)
	case brevity.PictureResponse:
		response = comp.ComposePictureResponse(c)
	case brevity.RadioCheckResponse:
		response = comp.ComposeRadioCheckResponse(c)
	case brevity.ShoppingResponse:
		response = comp.ComposeShoppingResponse(c)
	case brevity.SnaplockResponse:
		response = comp.ComposeSnaplockResponse(c)
	case brevity.SpikedResponseV2:
		response = comp.ComposeSpikedResponse(c)
	case brevity.StrobeResponse:
		response = comp.ComposeStrobeResponse(c)
	case brevity.TripwireResponse:
		response = comp.ComposeTripwireResponse(c)
	case brevity.VectorResponse:
		response = comp.ComposeVectorResponse(c)
	case brevity.SunriseCall:
		response = comp.ComposeSunriseCall(c)
	case brevity.ThreatCall:
		response = comp.ComposeThreatCall(c)
	case brevity.MergedCall:
		response = comp.ComposeMergedCall(c)
	case brevity.SayAgainResponse:
		response = comp.ComposeSayAgainResponse(c)
	default:
		logger.Debug().Msg("unable to route call to composition")
		a.trace(traces.WithRequestError(ctx, errors.New("no route for call")))
	}

	logger.Info().Str("speech", response.Speech).Str("subtitle", response.Subtitle).Msg("composed brevity call")
	if isImportantCall(call) {
		ctx = withImportant(ctx)
	}
	ctx = traces.WithCallText(ctx, response.Subtitle)
	ctx = traces.WithComposedAt(ctx, time.Now())
	out <- AsMessage(ctx, response)
}
