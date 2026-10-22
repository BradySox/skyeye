package composer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComposeCallsigns(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		callsigns []string
		expected  []string
	}{
		{
			callsigns: []string{"uzi 2 1"},
			expected:  []string{"Uzi 2 1"},
		},
		{
			callsigns: []string{"UZI 2 1"},
			expected:  []string{"Uzi 2 1"},
		},
		{
			callsigns: []string{"McFly 1"},
			expected:  []string{"McFly 1"},
		},
		{
			callsigns: []string{"Alpha 1"},
			expected:  []string{"Alpha 1"},
		},
		{
			callsigns: []string{"Alpha 1", "Bravo 1", "Charlie 1"},
			expected:  []string{"Alpha 1, Bravo 1, Charlie 1"},
		},
		{
			callsigns: []string{"Alpha 2", "Alpha 1", "Alpha 3"},
			expected:  []string{"Alpha 2, Alpha 1, Alpha 3"},
		},
		{
			callsigns: []string{"Charlie 1 1", "Alpha 1 2", "Bravo 1 2", "Bravo 1 1", "Alpha 1 1"},
			expected:  []string{"Charlie 1 1", "Alpha 1 2", "Bravo 1 2", "Bravo 1 1", "Alpha 1 1"},
		},
	}
	c := &Composer{"Tester"}
	for _, testCase := range testCases {
		t.Run(strings.Join(testCase.callsigns, ", "), func(t *testing.T) {
			t.Parallel()
			for _, expected := range testCase.expected {
				assert.Contains(t, c.composeCallsigns(testCase.callsigns...), expected)
			}
		})
	}
}
