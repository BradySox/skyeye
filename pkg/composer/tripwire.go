package composer

import (
	"github.com/dharmab/skyeye/pkg/brevity"
)

// ComposeTripwireResponse constructs natural language brevity for educating a caller about threat monitoring.
func (c *Composer) ComposeTripwireResponse(response brevity.TripwireResponse) NaturalLanguageResponse {
	reply := c.composeCallsigns(response.Callsign) + ", no tripwires needed. I watch every player on radar and call threats automatically. You can ask for PICTURE, BOGEY DOPE, SNAPLOCK, SPIKED, DECLARE or ALPHA CHECK."
	return NaturalLanguageResponse{
		Subtitle: reply,
		Speech:   reply,
	}
}
