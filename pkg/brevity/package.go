package brevity

// PackageRequestKind is what a pilot asks a package controller for.
type PackageRequestKind string

const (
	// PackageCheckIn is a flight checking in with its package controller.
	PackageCheckIn PackageRequestKind = "checkin"
	// PackageReadyToPush is a flight reporting ready to push.
	PackageReadyToPush PackageRequestKind = "ready"
	// PackagePushTime is a flight asking for the current push time.
	PackagePushTime PackageRequestKind = "pushtime"
	// PackageUnknown is a call to a package controller that names no request SkyEye knows. The
	// mission answers it with a say again in the controller's voice.
	PackageUnknown PackageRequestKind = "unknown"
)

// PackageRequest is a call to a package controller, such as an AWACS running a strike package.
// SkyEye does not answer it: it is passed to the mission over the mission link, which answers.
type PackageRequest struct {
	// Callsign of the pilot making the request.
	Callsign string
	// Controller is the package controller callsign the pilot called.
	Controller string
	// Kind of request.
	Kind PackageRequestKind
}

func (r PackageRequest) String() string {
	return "PACKAGE " + string(r.Kind) + " for " + r.Callsign + " to " + r.Controller
}
