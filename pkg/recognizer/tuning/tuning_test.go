package tuning

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptWithoutMission(t *testing.T) {
	t.Parallel()
	p := Prompt("Chainsaw", nil, Vocabulary{})
	assert.Contains(t, p, "Either ANYFACE or Chainsaw,")
	assert.NotContains(t, p, "Callsigns:")
	assert.NotContains(t, p, "Tower")
}

func TestPromptMissionVocabulary(t *testing.T) {
	t.Parallel()
	p := Prompt("Chainsaw", []string{"Tel Aviv"}, Vocabulary{
		Callsign:  "Sentinel",
		Callsigns: []string{"Blackjack", "Uzi", "uzi", " "},
		Fields:    []string{"Hatzerim", "Nevatim"},
	})
	assert.Contains(t, p, "Either ANYFACE or Sentinel,")
	assert.Contains(t, p, "Callsigns: Blackjack, Uzi.")
	assert.Contains(t, p, "Airfields: Hatzerim, Nevatim.")
	assert.Contains(t, p, "Locations: Tel Aviv.")
	assert.Contains(t, p, "ready for takeoff")
	// ATC gives no squawk codes, so the prompt does not steer toward one.
	assert.NotContains(t, strings.ToLower(p), "squawk")
	assert.Contains(t, p, "Check in")
	assert.Contains(t, p, "Off target, egressing.")
	// The mission's words come last, nearest the transcript.
	assert.True(t, strings.HasSuffix(p, "Airfields: Hatzerim, Nevatim."))
	assert.Less(t, strings.Index(p, "Locations:"), strings.Index(p, "Callsigns:"))
}

func TestPromptBudget(t *testing.T) {
	t.Parallel()
	many := make([]string, 0, 200)
	for range 200 {
		many = append(many, "Somewhere Very Long "+strconv.Itoa(len(many)))
	}
	p := Prompt("Chainsaw", many, Vocabulary{Callsigns: []string{"Blackjack"}, Fields: []string{"Hatzerim"}})
	assert.LessOrEqual(t, len(p), MaxPromptChars)
	// Locations give way first; the mission's words stay.
	assert.Contains(t, p, "Callsigns: Blackjack.")
	assert.Contains(t, p, "Airfields: Hatzerim.")
	assert.Contains(t, p, "Locations: Somewhere Very Long")
}

func TestPromptOneItemTooLong(t *testing.T) {
	t.Parallel()
	p := Prompt("Chainsaw", []string{strings.Repeat("y", MaxPromptChars)}, Vocabulary{})
	assert.LessOrEqual(t, len(p), MaxPromptChars)
	assert.NotContains(t, p, "Locations:")
}

func TestVocabularyIsZero(t *testing.T) {
	t.Parallel()
	assert.True(t, Vocabulary{}.IsZero())
	assert.False(t, Vocabulary{Fields: []string{"Hatzerim"}}.IsZero())
	assert.False(t, Vocabulary{Callsign: "Sentinel"}.IsZero())
}

func TestThreads(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		requested int
		numCPU    int
		expected  uint
	}{
		{requested: 0, numCPU: 28, expected: DefaultThreads},
		{requested: -3, numCPU: 28, expected: DefaultThreads},
		{requested: 4, numCPU: 28, expected: 4},
		{requested: 64, numCPU: 28, expected: 28},
		{requested: 0, numCPU: 4, expected: 4},
		{requested: 0, numCPU: 0, expected: 1},
	}
	for _, test := range testCases {
		assert.Equal(t, test.expected, Threads(test.requested, test.numCPU), "requested %d of %d CPUs", test.requested, test.numCPU)
	}
}
