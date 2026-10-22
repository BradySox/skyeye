package controller

import (
	"context"
	"time"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/dharmab/skyeye/pkg/traces"
	"github.com/lithammer/shortuuid/v3"
	"github.com/martinlindhe/unit"
)

func (c *Controller) broadcastSunrise(ctx context.Context) {
	radioFrequencies := c.srsClient.Frequencies()
	frequencies := make([]unit.Frequency, 0, len(radioFrequencies))
	for _, rf := range radioFrequencies {
		frequencies = append(frequencies, rf.Frequency)
	}
	c.calls <- NewCall(traces.WithTraceID(ctx, shortuuid.New()), brevity.SunriseCall{Frequencies: frequencies})
}

// HoldSunrise holds the sunrise call until ready closes, so it names the callsign the mission sets
// rather than the configured one. After timeout the call goes out anyway, and goes out again if
// ready closes later. Call it before Run.
func (c *Controller) HoldSunrise(ready <-chan struct{}, timeout time.Duration) {
	c.sunriseReady = ready
	c.sunriseTimeout = timeout
}

func (c *Controller) awaitSunrise(ctx context.Context, ready <-chan struct{}, timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ready:
		c.broadcastSunrise(ctx)
		return
	case <-timer.C:
		c.broadcastSunrise(ctx)
	case <-ctx.Done():
		return
	}
	select {
	case <-ready:
		c.broadcastSunrise(ctx)
	case <-ctx.Done():
	}
}
