package application

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/dharmab/skyeye/pkg/simpleradio"
	"github.com/rs/zerolog/log"
)

// maxBotVoices caps the SRS clients SkyEye opens for mission speakers. A speaker beyond the cap
// talks through the main client, under the GCI's name.
const maxBotVoices = 16

// botReadyWait is how long a call waits for a new speaker's client to log in to SRS before it goes
// out through the main client instead.
const botReadyWait = 5 * time.Second

// botRadio is the SRS client a mission speaker talks through.
type botRadio interface {
	AddListenRadios(frequencies []simpleradio.RadioFrequency) error
}

// botClaim is a frequency a mission speaker talks on, for mission lines that do not name their
// speaker.
type botClaim struct {
	frequency simpleradio.RadioFrequency
	speaker   string
}

// botVoices gives each mission speaker, such as "Hatzerim Ground", "Blackjack" or "Vacuum", its own
// SRS client, so the SRS client list and overlay show who is talking. The GCI's own calls, and
// every speaker past the cap, use the main client.
type botVoices[C botRadio] struct {
	// lock guards every field below.
	lock sync.Mutex
	// limit is the most speaker clients.
	limit int
	// create opens a speaker's client. Nil until the application runs.
	create func(name string, frequencies []simpleradio.RadioFrequency) (C, error)
	// main is the key of the GCI's own callsign, which speaks through the main client.
	main string
	// clients are the speaker clients by key.
	clients map[string]C
	// refused are the speakers that use the main client for good: their client failed, or the cap
	// was reached. Each is logged once.
	refused map[string]bool
	// claims are the frequencies each source of mission data says a speaker talks on.
	claims map[string][]botClaim
	// fallback is the one speaker on a frequency, from every source's claims. A frequency two
	// speakers claim is left out.
	fallback map[simpleradio.RadioFrequency]string
}

func newBotVoices[C botRadio](limit int) *botVoices[C] {
	return &botVoices[C]{
		limit:    limit,
		clients:  map[string]C{},
		refused:  map[string]bool{},
		claims:   map[string][]botClaim{},
		fallback: map[simpleradio.RadioFrequency]string{},
	}
}

// botSpeakerName is a speaker's name with its spacing cleaned up.
func botSpeakerName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

// botKey is a speaker's name as compared: "hatzerim ground".
func botKey(name string) string {
	return strings.ToLower(botSpeakerName(name))
}

// botClientName is a speaker's name in the SRS client list: "Hatzerim Ground [BOT]".
func botClientName(name string) string {
	return botSpeakerName(name) + " [BOT]"
}

// atcSpeakerName is the speaker for an airfield position: "Hatzerim Ground", "Hatzerim Tower".
func atcSpeakerName(field string, position string) string {
	position = strings.ToLower(strings.TrimSpace(position))
	if position == "" {
		return botSpeakerName(field)
	}
	return botSpeakerName(field + " " + strings.ToUpper(position[:1]) + position[1:])
}

// start lets speakers open their clients.
func (b *botVoices[C]) start(create func(name string, frequencies []simpleradio.RadioFrequency) (C, error)) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.create = create
}

// setMain names the GCI's own callsign, which always speaks through the main client.
func (b *botVoices[C]) setMain(name string) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.main = botKey(name)
}

// claim replaces the frequencies a source of mission data says its speakers talk on.
func (b *botVoices[C]) claim(source string, claims []botClaim) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.claims[source] = claims
	fallback := map[simpleradio.RadioFrequency]string{}
	shared := map[simpleradio.RadioFrequency]bool{}
	for _, list := range b.claims {
		for _, c := range list {
			if botKey(c.speaker) == "" || shared[c.frequency] {
				continue
			}
			if existing, ok := fallback[c.frequency]; ok && botKey(existing) != botKey(c.speaker) {
				delete(fallback, c.frequency)
				shared[c.frequency] = true
				continue
			}
			fallback[c.frequency] = botSpeakerName(c.speaker)
		}
	}
	b.fallback = fallback
}

// speakerOn is the one speaker the mission said talks on a frequency, or empty.
func (b *botVoices[C]) speakerOn(frequency simpleradio.RadioFrequency) string {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.fallback[frequency]
}

// ensure is the speaker's client, opened on first use and joined to the frequencies. False sends
// the speaker through the main client.
func (b *botVoices[C]) ensure(name string, frequencies []simpleradio.RadioFrequency) (C, bool) {
	var none C
	key := botKey(name)
	if key == "" {
		return none, false
	}
	b.lock.Lock()
	if key == b.main || b.refused[key] || b.create == nil {
		b.lock.Unlock()
		return none, false
	}
	client, ok := b.clients[key]
	if !ok {
		defer b.lock.Unlock()
		if len(b.clients) >= b.limit {
			b.refused[key] = true
			log.Warn().Str("speaker", name).Int("limit", b.limit).Msg("too many mission speakers, this one talks through the main SRS client")
			return none, false
		}
		created, err := b.create(botSpeakerName(name), frequencies)
		if err != nil {
			b.refused[key] = true
			log.Error().Err(err).Str("speaker", name).Msg("failed to open an SRS client for a mission speaker, it talks through the main SRS client")
			return none, false
		}
		b.clients[key] = created
		return created, true
	}
	b.lock.Unlock()
	if len(frequencies) > 0 {
		if err := client.AddListenRadios(frequencies); err != nil {
			log.Error().Err(err).Str("speaker", name).Msg("failed to tune a mission speaker's SRS client")
		}
	}
	return client, true
}

// pick is the client for a mission line: its speaker's, or when the line names none, the one
// speaker the mission said talks on its frequency. The name is the speaker picked.
func (b *botVoices[C]) pick(speaker string, frequency *simpleradio.RadioFrequency) (C, string, bool) {
	var frequencies []simpleradio.RadioFrequency
	if frequency != nil {
		frequencies = []simpleradio.RadioFrequency{*frequency}
		if botKey(speaker) == "" {
			speaker = b.speakerOn(*frequency)
		}
	}
	client, ok := b.ensure(speaker, frequencies)
	return client, botSpeakerName(speaker), ok
}

// refuse sends a speaker through the main client for good.
func (b *botVoices[C]) refuse(name string) {
	b.lock.Lock()
	defer b.lock.Unlock()
	key := botKey(name)
	delete(b.clients, key)
	b.refused[key] = true
}

// speakerKey carries the mission speaker a line is spoken as. Empty picks the speaker by the line's
// frequency.
type speakerKey struct{}

func withSpeaker(ctx context.Context, speaker string) context.Context {
	return context.WithValue(ctx, speakerKey{}, speaker)
}

func speakerFrom(ctx context.Context) (string, bool) {
	speaker, ok := ctx.Value(speakerKey{}).(string)
	return speaker, ok
}

// openBotClient opens a mission speaker's SRS client and runs it until the application stops.
func (a *Application) openBotClient(ctx context.Context, wg *sync.WaitGroup, name string, frequencies []simpleradio.RadioFrequency) (*simpleradio.Client, error) {
	client, err := a.srsClient.NewVoice(botClientName(name), frequencies)
	if err != nil {
		return nil, err
	}
	log.Info().Str("name", client.Name()).Int("frequencies", len(frequencies)).Msg("opened an SRS client for a mission speaker")
	wg.Go(func() {
		if err := client.Run(ctx, wg); err != nil && !errors.Is(err, context.Canceled) {
			log.Error().Err(err).Str("name", client.Name()).Msg("error running a mission speaker's SRS client")
		}
	})
	return client, nil
}

// botClientFor is the SRS client a call goes out under: its mission speaker's, or nil for the main
// client. Only mission lines have a speaker; the GCI's own calls never do.
func (a *Application) botClientFor(ctx context.Context) *simpleradio.Client {
	speaker, ok := speakerFrom(ctx)
	if !ok {
		return nil
	}
	var target *simpleradio.RadioFrequency
	if frequency, ok := frequencyFrom(ctx); ok {
		target = &frequency
	}
	client, name, ok := a.bots.pick(speaker, target)
	if !ok {
		return nil
	}
	if !client.WaitReady(botReadyWait) {
		log.Warn().Str("speaker", name).Msg("mission speaker's SRS client did not log in, it talks through the main SRS client")
		a.bots.refuse(name)
		return nil
	}
	return client
}

// applyATCVoices opens a client for each airfield position.
func (a *Application) applyATCVoices(atc []botClaim) {
	for _, c := range atc {
		a.bots.ensure(c.speaker, []simpleradio.RadioFrequency{c.frequency})
	}
	a.bots.claim("atc", atc)
}

// applyPackageVoices opens a client for each package controller, on the strike and package
// frequencies. A lone controller is the speaker of lines on strike that name none.
func (a *Application) applyPackageVoices(controllers []string, strike *simpleradio.RadioFrequency, frequencies []simpleradio.RadioFrequency) {
	if len(frequencies) > 0 {
		for _, name := range controllers {
			a.bots.ensure(name, frequencies)
		}
	}
	var claims []botClaim
	if strike != nil && len(controllers) == 1 {
		claims = append(claims, botClaim{frequency: *strike, speaker: controllers[0]})
	}
	a.bots.claim("packages", claims)
}

// applyTuneVoice opens a client for a tuned mission speaker, such as Vacuum, on its frequencies.
// Its own frequency is its own for lines that name no speaker.
func (a *Application) applyTuneVoice(callsign string, own *simpleradio.RadioFrequency, frequencies []simpleradio.RadioFrequency) {
	if len(frequencies) > 0 {
		a.bots.ensure(callsign, frequencies)
	}
	var claims []botClaim
	if own != nil {
		claims = append(claims, botClaim{frequency: *own, speaker: callsign})
	}
	a.bots.claim("tune:"+botKey(callsign), claims)
}
