package brevity

// ELINTRequestKind is what a pilot asks a mission's ELINT speaker, such as Vacuum, for.
type ELINTRequestKind string

const (
	// ELINTCheckIn is a flight checking in: the speaker answers with how many radars are on.
	ELINTCheckIn ELINTRequestKind = "checkin"
	// ELINTEmitters asks for the nearest radiating radars by bearing and range from the caller.
	ELINTEmitters ELINTRequestKind = "emitters"
	// ELINTTarget asks which radars are radiating around the caller's package target.
	ELINTTarget ELINTRequestKind = "target"
	// ELINTMud asks which radar is radiating on a bearing from the caller, as a RWR shows it.
	ELINTMud ELINTRequestKind = "mud"
	// ELINTSayAgain asks the speaker to repeat its last call.
	ELINTSayAgain ELINTRequestKind = "sayagain"
	// ELINTUnknown is a call to the speaker that names no request SkyEye knows.
	ELINTUnknown ELINTRequestKind = "unknown"
)

// ELINTRequest is a call to a mission's ELINT speaker. SkyEye does not answer it: it is passed to
// the mission over the mission link, which knows the radars and decides the reply.
type ELINTRequest struct {
	// Callsign of the pilot, if heard.
	Callsign string
	// Speaker is the callsign called, such as "Vacuum".
	Speaker string
	// Kind of request.
	Kind ELINTRequestKind
	// Bearing is the magnetic bearing for a MUD request, such as "270". Empty if none was heard.
	Bearing string
}

func (r ELINTRequest) String() string {
	return "ELINT " + string(r.Kind) + " for " + r.Callsign + " to " + r.Speaker
}
