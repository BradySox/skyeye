package speakers

import (
	"errors"
	"time"
)

// NewDiskSpeaker is not available on macOS, which speaks with the system voice.
func NewDiskSpeaker(_, _ string, _ float64, _ time.Duration) (Speaker, error) {
	return nil, errors.New("disk voices are not supported on macOS")
}
