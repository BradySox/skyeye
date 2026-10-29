package composer

import (
	"github.com/dharmab/skyeye/pkg/brevity"
)

// ComposeCheckInResponse constructs natural language brevity for responding to an ambiguous CHECK IN call.
func (c *Composer) ComposeCheckInResponse(response brevity.CheckInResponse) NaturalLanguageResponse {
	reply := c.composeCallsigns(response.Callsign) + ", copy your check in."
	return NaturalLanguageResponse{
		Subtitle: reply,
		Speech:   reply,
	}
}
