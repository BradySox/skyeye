//go:build !darwin

package speakers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/dharmab/skyeye/pkg/pcm"
	"github.com/dharmab/skyeye/pkg/synthesizer/voices"
	"github.com/martinlindhe/unit"
)

// diskSynth speaks with a Piper voice model read from disk, optionally one speaker of a
// multi-speaker model. It runs the Piper binary the embedded voices install.
type diskSynth struct {
	exe         string
	model       string
	config      string
	speaker     int
	rate        unit.Frequency
	speed       float64
	pauseLength time.Duration
}

var _ Speaker = (*diskSynth)(nil)

// voiceConfig is the part of a Piper .onnx.json SkyEye reads.
type voiceConfig struct {
	Audio struct {
		SampleRate int `json:"sample_rate"`
	} `json:"audio"`
	SpeakerIDMap map[string]int `json:"speaker_id_map"`
}

// NewDiskSpeaker creates a Speaker from a Piper voice under voicesDir. name is the voice folder,
// such as "en_GB-alan-medium", optionally followed by "#" and a speaker of a multi-speaker
// model, such as "en_US-l2arctic-medium#ABA".
func NewDiskSpeaker(voicesDir, name string, playbackSpeed float64, playbackPause time.Duration) (Speaker, error) {
	folder, speakerName, _ := strings.Cut(name, "#")
	dir := filepath.Join(voicesDir, folder)
	model := filepath.Join(dir, folder+".onnx")
	config := model + ".json"
	b, err := os.ReadFile(config)
	if err != nil {
		return nil, fmt.Errorf("failed to read voice %q: %w", name, err)
	}
	var vc voiceConfig
	if err := json.Unmarshal(b, &vc); err != nil {
		return nil, fmt.Errorf("failed to parse voice %q: %w", name, err)
	}
	speaker := 0
	if speakerName != "" {
		id, ok := vc.SpeakerIDMap[speakerName]
		if !ok {
			n, convErr := strconv.Atoi(speakerName)
			if convErr != nil {
				return nil, fmt.Errorf("voice %q has no speaker %q", folder, speakerName)
			}
			id = n
		}
		speaker = id
	}
	rate := vc.Audio.SampleRate
	if rate == 0 {
		rate = 22050
	}
	exe, err := piperBinary()
	if err != nil {
		return nil, err
	}
	return &diskSynth{
		exe:         exe,
		model:       model,
		config:      config,
		speaker:     speaker,
		rate:        unit.Frequency(rate) * unit.Hertz,
		speed:       playbackSpeed,
		pauseLength: playbackPause,
	}, nil
}

// piperBinary is the Piper executable that the embedded voices install, installing it if needed.
func piperBinary() (string, error) {
	if _, err := NewPiperSpeaker(voices.FeminineVoice, 1, 0); err != nil {
		return "", fmt.Errorf("failed to install Piper: %w", err)
	}
	dataDir, err := xdg.DataFile("ab-piper")
	if err != nil {
		return "", fmt.Errorf("failed to find Piper data dir: %w", err)
	}
	exe := "piper"
	if runtime.GOOS == "windows" {
		exe = "piper.exe"
	}
	path := filepath.Join(dataDir, "piper-bin-"+runtime.GOOS, exe)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("piper binary not found at %s: %w", path, err)
	}
	return path, nil
}

// SayContext implements [Speaker.SayContext].
func (s *diskSynth) SayContext(ctx context.Context, text string) ([]float32, error) {
	tmpDir, err := os.MkdirTemp("", "skyeye-piper.")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	out := filepath.Join(tmpDir, "tts.wav")
	args := []string{
		"--model", s.model,
		"--config", s.config,
		"--speaker", strconv.Itoa(s.speaker),
		"--output_file", out,
		"--sentence_silence", fmt.Sprintf("%f", s.pauseLength.Seconds()),
	}
	if s.speed != 1.0 {
		args = append(args, "--length_scale", fmt.Sprintf("%f", s.speed))
	}
	// The binary is the one Piper installs and the model paths come from SkyEye's own voices folder.
	cmd := exec.CommandContext(ctx, s.exe, args...) //nolint:gosec
	cmd.Dir = filepath.Dir(s.exe)
	cmd.Stdin = strings.NewReader(text)
	stderr := bytes.NewBuffer(nil)
	cmd.Stderr = stderr
	hideWindow(cmd)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to synthesize text: %w: %s", err, stderr.Bytes())
	}
	wav, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("failed to read synthesized audio: %w", err)
	}
	if len(wav) == 0 {
		return nil, errors.New("piper produced no audio")
	}
	downsampled, err := downsample(wav, s.rate)
	if err != nil {
		return nil, fmt.Errorf("failed to downsample synthesized audio: %w", err)
	}
	return pcm.S16LEBytesToF32LE(downsampled), nil
}

// Say implements [Speaker.Say].
func (s *diskSynth) Say(text string) ([]float32, error) {
	return s.SayContext(context.Background(), text)
}
