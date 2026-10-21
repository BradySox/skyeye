//go:build !darwin

package speakers

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/dharmab/skyeye/pkg/pcm"
	"github.com/martinlindhe/unit"
)

// kokoroSpeakers is the speaker order of the Kokoro v1.0 voices.bin that sherpa-onnx publishes.
var kokoroSpeakers = strings.Split("af_alloy,af_aoede,af_bella,af_heart,af_jessica,af_kore,af_nicole,"+
	"af_nova,af_river,af_sarah,af_sky,am_adam,am_echo,am_eric,am_fenrir,am_liam,am_michael,am_onyx,"+
	"am_puck,am_santa,bf_alice,bf_emma,bf_isabella,bf_lily,bm_daniel,bm_fable,bm_george,bm_lewis,"+
	"ef_dora,em_alex,ff_siwis,hf_alpha,hf_beta,hm_omega,hm_psi,if_sara,im_nicola,jf_alpha,"+
	"jf_gongitsune,jf_nezumi,jf_tebukuro,jm_kumo,pf_dora,pm_alex,pm_santa,zf_xiaobei,zf_xiaoni,"+
	"zf_xiaoxiao,zf_xiaoyi,zm_yunjian,zm_yunxi,zm_yunxia,zm_yunyang,em_santa", ",")

// kokoroSynth speaks with one Kokoro voice through sherpa-onnx's offline TTS tool, which lives in
// the model folder's bin directory.
type kokoroSynth struct {
	exe     string
	dir     string
	speaker int
	british bool
	speed   float64
}

var _ Speaker = (*kokoroSynth)(nil)

// NewKokoroSpeaker creates a Speaker for a Kokoro voice such as "am_eric" from the model in dir.
func NewKokoroSpeaker(dir, voice string, playbackSpeed float64) (Speaker, error) {
	speaker := slices.Index(kokoroSpeakers, voice)
	if speaker < 0 {
		return nil, fmt.Errorf("kokoro has no voice %q", voice)
	}
	exe := "sherpa-onnx-offline-tts"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	exe = filepath.Join(dir, "bin", exe)
	for _, required := range []string{exe, filepath.Join(dir, "model.onnx"), filepath.Join(dir, "voices.bin")} {
		if _, err := os.Stat(required); err != nil {
			return nil, fmt.Errorf("kokoro voice %q needs %s: %w", voice, required, err)
		}
	}
	return &kokoroSynth{
		exe:     exe,
		dir:     dir,
		speaker: speaker,
		british: strings.HasPrefix(voice, "b"),
		speed:   playbackSpeed,
	}, nil
}

func (s *kokoroSynth) args(text, out string) []string {
	lang, lexicon := "en-us", "lexicon-us-en.txt"
	if s.british {
		// The espeak-ng data names British English "en"; "en-gb" fails to load.
		lang, lexicon = "en", "lexicon-gb-en.txt"
	}
	args := []string{
		"--kokoro-model=" + filepath.Join(s.dir, "model.onnx"),
		"--kokoro-voices=" + filepath.Join(s.dir, "voices.bin"),
		"--kokoro-tokens=" + filepath.Join(s.dir, "tokens.txt"),
		"--kokoro-data-dir=" + filepath.Join(s.dir, "espeak-ng-data"),
		"--kokoro-lexicon=" + filepath.Join(s.dir, lexicon),
		"--kokoro-lang=" + lang,
		"--num-threads=4",
		"--sid=" + strconv.Itoa(s.speaker),
		"--output-filename=" + out,
	}
	if s.speed != 1.0 && s.speed > 0 {
		args = append(args, fmt.Sprintf("--kokoro-length-scale=%f", s.speed))
	}
	return append(args, text)
}

// wavPCM returns the 16-bit PCM data and sample rate of a canonical WAV file.
func wavPCM(wav []byte) ([]byte, unit.Frequency, error) {
	if len(wav) < 12 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, 0, errors.New("not a WAV file")
	}
	var rate uint32
	for offset := 12; offset+8 <= len(wav); {
		id := string(wav[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(wav[offset+4 : offset+8]))
		body := offset + 8
		switch id {
		case "fmt ":
			if body+8 <= len(wav) {
				rate = binary.LittleEndian.Uint32(wav[body+4 : body+8])
			}
		case "data":
			end := min(body+size, len(wav))
			if rate == 0 {
				return nil, 0, errors.New("WAV data before its format")
			}
			return wav[body:end], unit.Frequency(rate) * unit.Hertz, nil
		}
		offset = body + size + size%2
	}
	return nil, 0, errors.New("WAV file has no data")
}

// SayContext implements [Speaker.SayContext].
func (s *kokoroSynth) SayContext(ctx context.Context, text string) ([]float32, error) {
	tmpDir, err := os.MkdirTemp("", "skyeye-kokoro.")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	out := filepath.Join(tmpDir, "tts.wav")
	// The tool is the one in SkyEye's own voices folder.
	cmd := exec.CommandContext(ctx, s.exe, s.args(text, out)...) //nolint:gosec
	cmd.Dir = filepath.Dir(s.exe)
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
	samples, rate, err := wavPCM(wav)
	if err != nil {
		return nil, err
	}
	downsampled, err := downsample(samples, rate)
	if err != nil {
		return nil, fmt.Errorf("failed to downsample synthesized audio: %w", err)
	}
	return pcm.S16LEBytesToF32LE(downsampled), nil
}

// Say implements [Speaker.Say].
func (s *kokoroSynth) Say(text string) ([]float32, error) {
	return s.SayContext(context.Background(), text)
}
