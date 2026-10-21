package recognizer

type recognizerOptions struct {
	locations []string
	threads   uint
}

// Option configures a recognizer.
type Option func(*recognizerOptions)

// WithLocations adds location names to the recognizer's initial prompt,
// improving transcription accuracy for place names.
func WithLocations(locations []string) Option {
	return func(o *recognizerOptions) {
		o.locations = locations
	}
}

// WithThreads sets the CPU threads a local recognizer uses. Zero keeps the library's default.
func WithThreads(threads uint) Option {
	return func(o *recognizerOptions) {
		o.threads = threads
	}
}
