package brevity

// ATCRequestKind is what a pilot asks an airfield controller for.
type ATCRequestKind string

const (
	// ATCStartup is a request for engine start.
	ATCStartup ATCRequestKind = "startup"
	// ATCTaxi is a request for taxi.
	ATCTaxi ATCRequestKind = "taxi"
	// ATCReady is a flight holding short, ready for takeoff.
	ATCReady ATCRequestKind = "ready"
	// ATCAirborne is a flight reporting airborne.
	ATCAirborne ATCRequestKind = "airborne"
	// ATCInbound is a flight inbound to land or in the pattern: "10 miles, full stop", "initial",
	// "base, gear down".
	ATCInbound ATCRequestKind = "inbound"
	// ATCClear is a flight clear of the runway after landing.
	ATCClear ATCRequestKind = "clear"
	// ATCEmergency is a mayday, or an engine failure or fire called without one.
	ATCEmergency ATCRequestKind = "emergency"
	// ATCRadioCheck is a radio check on an airfield frequency.
	ATCRadioCheck ATCRequestKind = "radiocheck"
	// ATCRunwayInUse asks for the runway in use: "request runway in use", "say active".
	ATCRunwayInUse ATCRequestKind = "runway"
	// ATCSayAgain asks the controller to repeat its last call.
	ATCSayAgain ATCRequestKind = "sayagain"
	// ATCReadback is a pilot reading back a clearance: the items, then the callsign.
	ATCReadback ATCRequestKind = "readback"
	// ATCUnknown is a call to the controller that names no request SkyEye knows.
	ATCUnknown ATCRequestKind = "unknown"
)

// ATCRequest is a call heard on an airfield frequency. SkyEye does not answer it: it is passed to
// the mission over the mission link, which decides the reply.
type ATCRequest struct {
	// Callsign of the pilot, if heard.
	Callsign string
	// Position is the controller the pilot called: "ground", "tower", and so on. Empty for a
	// readback, which names no controller.
	Position string
	// Kind of request.
	Kind ATCRequestKind
	// Detail qualifies the kind. Inbound: "overhead", "straightin", "initial", "break" or
	// "geardown" (base or final, the landing call). Taxi or ready: "numbers" ("have the numbers").
	Detail string
	// Numbers are the numbers heard, in order, without the callsign's: a runway or
	// a frequency such as "270.45". The mission checks a readback against them.
	Numbers []string
	// Text is the normalized call, for the mission to check words such as a taxiway name.
	Text string
}

func (r ATCRequest) String() string {
	return "ATC " + string(r.Kind) + " for " + r.Callsign + " to " + r.Position
}
