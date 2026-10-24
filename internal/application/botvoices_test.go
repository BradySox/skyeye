package application

import (
	"errors"
	"testing"

	"github.com/dharmab/skyeye/pkg/simpleradio"
	"github.com/stretchr/testify/require"
)

// fakeBotRadio records what a speaker's client was tuned to.
type fakeBotRadio struct {
	name  string
	tuned []simpleradio.RadioFrequency
}

func (f *fakeBotRadio) AddListenRadios(frequencies []simpleradio.RadioFrequency) error {
	f.tuned = append(f.tuned, frequencies...)
	return nil
}

func testFrequency(t *testing.T, s string) simpleradio.RadioFrequency {
	t.Helper()
	frequency, err := simpleradio.ParseRadioFrequency(s)
	require.NoError(t, err)
	return *frequency
}

// newTestBotVoices is a started registry whose clients are fakes; created counts the clients
// opened.
func newTestBotVoices(limit int) (*botVoices[*fakeBotRadio], *[]string) {
	b := newBotVoices[*fakeBotRadio](limit)
	created := &[]string{}
	b.start(func(name string, frequencies []simpleradio.RadioFrequency) (*fakeBotRadio, error) {
		*created = append(*created, name)
		return &fakeBotRadio{name: name, tuned: frequencies}, nil
	})
	return b, created
}

func TestBotNames(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Hatzerim Ground [BOT]", botClientName("Hatzerim Ground"))
	require.Equal(t, "Image [BOT]", botClientName("  Image "))
	require.Equal(t, "hatzerim ground", botKey("Hatzerim  Ground"))
	require.Equal(t, "Hatzerim Ground", atcSpeakerName("Hatzerim", "ground"))
	require.Equal(t, "Hatzerim Tower", atcSpeakerName("Hatzerim", "Tower"))
	require.Equal(t, "Ramat David Approach", atcSpeakerName("Ramat David", "approach"))
	require.Equal(t, "Hatzerim", atcSpeakerName("Hatzerim", ""))
}

func TestBotVoicesPickBySpeaker(t *testing.T) {
	t.Parallel()
	b, created := newTestBotVoices(maxBotVoices)
	ground := testFrequency(t, "270.450AM")
	tower := testFrequency(t, "251.300AM")

	client, name, ok := b.pick("Hatzerim Ground", &ground)
	require.True(t, ok)
	require.Equal(t, "Hatzerim Ground", name)
	require.Equal(t, "Hatzerim Ground", client.name)

	again, _, ok := b.pick("hatzerim ground", &tower)
	require.True(t, ok)
	require.Same(t, client, again, "a speaker keeps its one client")
	require.Equal(t, []simpleradio.RadioFrequency{ground, tower}, client.tuned, "a new frequency is joined")
	require.Equal(t, []string{"Hatzerim Ground"}, *created)
}

func TestBotVoicesGCIUsesTheMainClient(t *testing.T) {
	t.Parallel()
	b, created := newTestBotVoices(maxBotVoices)
	b.setMain("Image")
	_, _, ok := b.pick("image", nil)
	require.False(t, ok)
	_, _, ok = b.pick("", nil)
	require.False(t, ok, "a line with no speaker and no frequency")
	require.Empty(t, *created)
}

// A line that names no speaker goes out under the one speaker the mission said talks on its
// frequency; a frequency two speakers share goes out under the GCI.
func TestBotVoicesFallbackByFrequency(t *testing.T) {
	t.Parallel()
	b, _ := newTestBotVoices(maxBotVoices)
	ground := testFrequency(t, "270.450AM")
	strike := testFrequency(t, "264.500AM")
	other := testFrequency(t, "305.000AM")
	b.claim("atc", []botClaim{{frequency: ground, speaker: "Hatzerim Ground"}})
	b.claim("packages", []botClaim{{frequency: strike, speaker: "Blackjack"}})
	require.Equal(t, "Blackjack", b.speakerOn(strike))

	b.claim("tune:vacuum", []botClaim{{frequency: strike, speaker: "Vacuum"}})
	require.Empty(t, b.speakerOn(strike), "two speakers share strike")
	require.Equal(t, "Hatzerim Ground", b.speakerOn(ground))

	client, name, ok := b.pick("", &ground)
	require.True(t, ok)
	require.Equal(t, "Hatzerim Ground", name)
	require.Equal(t, "Hatzerim Ground", client.name)
	_, _, ok = b.pick("", &strike)
	require.False(t, ok)
	_, _, ok = b.pick("", &other)
	require.False(t, ok)

	b.claim("tune:vacuum", nil)
	require.Equal(t, "Blackjack", b.speakerOn(strike), "a source's claims are replaced")
	_, name, ok = b.pick("Vacuum", &strike)
	require.True(t, ok, "a named speaker is used on a shared frequency")
	require.Equal(t, "Vacuum", name)
}

func TestBotVoicesCap(t *testing.T) {
	t.Parallel()
	b, created := newTestBotVoices(2)
	frequency := testFrequency(t, "251.000AM")
	for _, speaker := range []string{"Hatzerim Ground", "Hatzerim Tower"} {
		_, _, ok := b.pick(speaker, &frequency)
		require.True(t, ok)
	}
	_, _, ok := b.pick("Vacuum", &frequency)
	require.False(t, ok, "past the cap a speaker uses the main client")
	_, _, ok = b.pick("Hatzerim Tower", &frequency)
	require.True(t, ok, "speakers under the cap keep their clients")

	b.refuse("Hatzerim Tower")
	_, _, ok = b.pick("Hatzerim Tower", &frequency)
	require.False(t, ok, "a refused speaker uses the main client")
	_, _, ok = b.pick("Vacuum", &frequency)
	require.False(t, ok, "a speaker over the cap stays on the main client")
	require.Equal(t, []string{"Hatzerim Ground", "Hatzerim Tower"}, *created)
}

func TestBotVoicesCreateFailure(t *testing.T) {
	t.Parallel()
	b := newBotVoices[*fakeBotRadio](maxBotVoices)
	frequency := testFrequency(t, "251.000AM")
	_, _, ok := b.pick("Vacuum", &frequency)
	require.False(t, ok, "no clients before the application runs")

	attempts := 0
	b.start(func(string, []simpleradio.RadioFrequency) (*fakeBotRadio, error) {
		attempts++
		return nil, errors.New("connection refused")
	})
	_, _, ok = b.pick("Vacuum", &frequency)
	require.False(t, ok)
	_, _, ok = b.pick("Vacuum", &frequency)
	require.False(t, ok)
	require.Equal(t, 1, attempts, "a failed speaker is not retried")
}

func TestSpeakerContext(t *testing.T) {
	t.Parallel()
	_, ok := speakerFrom(t.Context())
	require.False(t, ok, "the GCI's own calls carry no speaker")
	speaker, ok := speakerFrom(withSpeaker(t.Context(), ""))
	require.True(t, ok, "a mission line with no speaker still picks one by frequency")
	require.Empty(t, speaker)
	speaker, ok = speakerFrom(withSpeaker(t.Context(), "Vacuum"))
	require.True(t, ok)
	require.Equal(t, "Vacuum", speaker)
}
