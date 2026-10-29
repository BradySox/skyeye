package application

import (
	"os"
	"path/filepath"

	"github.com/dharmab/skyeye/pkg/missionlink"
	"github.com/dharmab/skyeye/pkg/simpleradio"
	"github.com/rs/zerolog/log"
)

// recordingDir is the configured recording folder, or Documents\Tacview, where Tacview keeps its
// recordings: its auto-load add-on only finds audio next to the .acmi.
func recordingDir(configured string) string {
	if configured != "" {
		return configured
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "recordings"
	}
	return filepath.Join(home, "Documents", "Tacview")
}

// applyRecord records the frequencies a mission asks for.
func (a *Application) applyRecord(record missionlink.Record) {
	frequencies := make([]simpleradio.RadioFrequency, 0, len(record.Frequencies))
	recordOnly := make([]bool, 0, len(record.Frequencies))
	labels := make(map[string]string, len(record.Frequencies))
	for _, f := range record.Frequencies {
		frequency, err := simpleradio.ParseRadioFrequency(f.Frequency)
		if err != nil {
			log.Warn().Err(err).Str("frequency", f.Frequency).Msg("ignoring unreadable record frequency")
			continue
		}
		frequencies = append(frequencies, *frequency)
		recordOnly = append(recordOnly, f.RecordOnly)
		labels[frequency.String()] = f.Label
	}
	a.recorder.SetFrequencies(labels)
	if err := a.srsClient.RecordRadios(frequencies, recordOnly); err != nil {
		log.Error().Err(err).Msg("failed to tune recorded frequencies")
	}
	log.Info().Int("frequencies", len(frequencies)).Msg("recording radios for Tacview")
}
