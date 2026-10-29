// Command sttsweep finds calls SkyEye mishears. "list" prints the test calls as JSON lines; a
// script speaks each one with text-to-speech and degrades it to radio quality. "run" sends the
// clips through SkyEye's own Whisper recognizer and parser, and writes a Markdown report of every
// call whose transcript parses differently from the script.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dharmab/skyeye/pkg/parser"
	"github.com/dharmab/skyeye/pkg/recognizer"
	"github.com/dharmab/skyeye/pkg/recognizer/tuning"
	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/gopxl/beep/v2/wav"
)

// call is one test transmission. Route is the frequency it is heard on: gci, package, elint or atc.
type call struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Route   string `json:"route"`
	GCI     string `json:"gci"`
	Theater string `json:"theater"`
}

var (
	gcis        = []string{"Chainsaw", "Chalice", "Image"}
	controllers = []string{"Ramrod", "Anvil", "Blackjack", "Crown", "Sentinel"}
	flights     = []string{"Enfield four one", "Springfield two", "Uzi two one", "Colt one one", "Dodge four", "Ford nine one", "Chevy two", "Pontiac three one"}
	theaters    = map[string][]string{
		"germany":     {"Spangdahlem", "Ramstein", "Bitburg", "Hahn"},
		"israel":      {"Hatzerim", "Nevatim", "Ramat David", "Tel Nof"},
		"afghanistan": {"Shindand", "Kandahar", "Bagram"},
		"caucasus":    {"Kobuleti", "Batumi", "Senaki"},
		"syria":       {"Incirlik", "Akrotiri"},
	}
	theaterOrder = []string{"germany", "israel", "afghanistan", "caucasus", "syria"}
)

var (
	gciCalls = []string{
		"{G}, {F}, radio check.", "{G}, {F}, request alpha check.", "{G}, {F}, bogey dope.",
		"{G}, {F}, request picture.", "{G}, {F}, declare, bullseye one eight zero, twenty five, angels twenty.",
		"{G}, {F}, spiked, two seven zero.", "{G}, {F}, strobe, zero niner zero.", "{G}, {F}, snaplock, bullseye zero four five, thirty, angels fifteen.",
		"{G}, {F}, vector to tanker.", "{G}, {F}, checking in.", "{G}, {F}, bogey dope, fighters.",
	}
	packageCalls = []string{
		"{P}, {F}, checking in.", "{P}, {F}, check in.", "{P}, {F}, ready to push.", "{P}, {F}, say push time.",
		"{P}, {F}, airborne.", "{P}, {F}, heading to hold.", "{P}, {F}, off target.", "{P}, {F}, egressing.",
		"{P}, {F}, ready.", "{P}, {F}, airborne, proceeding to hold.",
	}
	elintCalls = []string{
		"Vacuum, {F}, checking in.", "Vacuum, {F}, say emitters.", "Vacuum, {F}, SAM emitters.",
		"Vacuum, {F}, mud, SA six, one eight zero.", "Vacuum, {F}, say again.",
	}
	atcCalls = []string{
		"{A} Ground, {F}, radio check.", "{A} Ground, {F}, ready to taxi.", "{A} Ground, {F}, request engine start.",
		"{A} Tower, {F}, holding short, runway two three.", "{A} Tower, {F}, ready for takeoff.",
		"{A} Tower, {F}, airborne.", "{A} Tower, {F}, inbound, thirty miles, have the numbers.",
		"{A} Tower, {F}, initial.", "{A} Tower, {F}, base, gear down.", "{A} Tower, {F}, request runway in use.",
		"{A} Tower, {F}, declaring a fuel emergency.", "{A} Tower, {F}, full stop.", "{F}, ready to taxi.",
		"{A}, {F}, ready to taxi.", "Mayday, mayday, {F}, engine failure.", "{A} Ground, {F}, flight of two, request taxi.",
	}
)

func calls() []call {
	var out []call
	n := 0
	add := func(route, template, gci, theater string) {
		field := theaters[theater][n%len(theaters[theater])]
		text := strings.NewReplacer(
			"{G}", gci,
			"{P}", controllers[n%len(controllers)],
			"{F}", flights[n%len(flights)],
			"{A}", field,
		).Replace(template)
		out = append(out, call{ID: fmt.Sprintf("c%03d", n), Text: text, Route: route, GCI: gci, Theater: theater})
		n++
	}
	for i, t := range gciCalls {
		for j, g := range gcis {
			add("gci", t, g, theaterOrder[(i+j)%len(theaterOrder)])
		}
	}
	for i, t := range packageCalls {
		for j := range 3 {
			add("package", t, gcis[(i+j)%len(gcis)], theaterOrder[(i+j)%len(theaterOrder)])
		}
	}
	for i, t := range elintCalls {
		for j := range 3 {
			add("elint", t, gcis[(i+j)%len(gcis)], theaterOrder[(i+j)%len(theaterOrder)])
		}
	}
	for i, t := range atcCalls {
		for j := range 3 {
			add("atc", t, gcis[(i+j)%len(gcis)], theaterOrder[(i+j)%len(theaterOrder)])
		}
	}
	return out
}

// vocabulary is what a RetLab mission in the theater tells SkyEye.
func vocabulary(c call) tuning.Vocabulary {
	callsigns := slices(controllers, []string{"Vacuum"})
	for _, f := range flights {
		callsigns = append(callsigns, strings.Fields(f)[0])
	}
	return tuning.Vocabulary{Callsign: c.GCI, Callsigns: callsigns, Fields: theaters[c.Theater]}
}

func slices(a, b []string) []string {
	return append(append([]string{}, a...), b...)
}

// parse reads text as SkyEye's application does on the call's frequency.
func parse(c call, text string) string {
	p := parser.New(c.GCI, []string{}, false)
	p.SetPackageControllers(controllers)
	p.SetSpeaker("Vacuum")
	var request any
	switch c.Route {
	case "atc":
		if r := parser.ParseATC(parser.TrimFieldName(text, theaters[c.Theater]), true); r != nil {
			// The request carries the text it was read from, which differs by design.
			r.Text = ""
			request = *r
		}
	case "elint":
		if r := p.ParseELINT(text, "Vacuum"); r != nil {
			request = *r
		}
	default:
		request = p.Parse(text)
		if request == nil && c.Route == "package" {
			if r := p.ParseLoosePackageCall(text); r != nil {
				request = *r
			}
		}
	}
	if request == nil {
		return "nothing"
	}
	b, _ := json.Marshal(request)
	return fmt.Sprintf("%T %s", request, b)
}

func loadPCM(path string) ([]float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	streamer, format, err := wav.Decode(f)
	if err != nil {
		return nil, err
	}
	defer streamer.Close()
	if format.SampleRate != 16000 || format.NumChannels != 1 {
		return nil, fmt.Errorf("%s: want 16 kHz mono, got %d Hz %d channels", path, format.SampleRate, format.NumChannels)
	}
	var pcm []float32
	buf := make([][2]float64, 512)
	for {
		n, ok := streamer.Stream(buf)
		for i := range n {
			pcm = append(pcm, float32(buf[i][0]))
		}
		if !ok {
			return pcm, streamer.Err()
		}
	}
}

func run(modelPath, clipDir, reportPath string) error {
	model, err := whisper.New(modelPath)
	if err != nil {
		return err
	}
	defer model.Close()
	byID := map[string]call{}
	for _, c := range calls() {
		byID[c.ID] = c
	}
	recognizers := map[string]recognizer.Recognizer{}
	clips, err := filepath.Glob(filepath.Join(clipDir, "*.wav"))
	if err != nil {
		return err
	}
	sort.Strings(clips)
	type miss struct{ voice, text, heard, want, got string }
	var misses []miss
	total := 0
	for _, clip := range clips {
		// Clips are named <call id>_<voice>.wav.
		base := strings.TrimSuffix(filepath.Base(clip), ".wav")
		id, voice, _ := strings.Cut(base, "_")
		c, ok := byID[id]
		if !ok {
			continue
		}
		r, ok := recognizers[c.GCI+c.Theater]
		if !ok {
			r = recognizer.NewWhisperRecognizer(&model, c.GCI)
			r.(recognizer.VocabularySetter).SetVocabulary(vocabulary(c))
			recognizers[c.GCI+c.Theater] = r
		}
		pcm, err := loadPCM(clip)
		if err != nil {
			return err
		}
		heard, err := r.Recognize(context.Background(), pcm, false)
		if err != nil {
			return err
		}
		heard = strings.TrimSpace(heard)
		total++
		want, got := parse(c, c.Text), parse(c, heard)
		if want != got {
			misses = append(misses, miss{voice, c.Text, heard, want, got})
		}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# SkyEye speech sweep\n\n%d clips, %d parsed differently from the script.\n\n", total, len(misses))
	sb.WriteString("| Voice | Said | Heard | Script parses as | Heard parses as |\n|---|---|---|---|---|\n")
	for _, m := range misses {
		fmt.Fprintf(&sb, "| %s | %s | %s | `%s` | `%s` |\n", m.voice, m.text, strings.ReplaceAll(m.heard, "|", "/"), m.want, m.got)
	}
	return os.WriteFile(reportPath, []byte(sb.String()), 0o644)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "list" {
		w := bufio.NewWriter(os.Stdout)
		defer w.Flush()
		enc := json.NewEncoder(w)
		for _, c := range calls() {
			_ = enc.Encode(c)
		}
		return
	}
	if len(os.Args) == 5 && os.Args[1] == "run" {
		if err := run(os.Args[2], os.Args[3], os.Args[4]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "usage: sttsweep list | sttsweep run <model.bin> <clip dir> <report.md>")
	os.Exit(2)
}
