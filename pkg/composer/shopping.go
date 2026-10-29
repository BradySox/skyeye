package composer

import (
	"github.com/dharmab/skyeye/pkg/brevity"
)

// ComposeShoppingResponse constructs natural language brevity for educating a caller about SHOPPING brevity.
func (c *Composer) ComposeShoppingResponse(r brevity.ShoppingResponse) NaturalLanguageResponse {
	reply := c.composeCallsigns(r.Callsign) + ", SHOPPING is an air-to-surface call for a JTAC or FAC. For air threats, ask for BOGEY DOPE or PICTURE."
	return NaturalLanguageResponse{
		Subtitle: reply,
		Speech:   reply,
	}
}
