package parser

import "strings"

// offTargetWords report a flight off the target: "Ramrod, Colt 1-1, off target", "off",
// "egressing", "off and egressing". Read after the airborne report, so "off the ground" stays one.
var offTargetWords = []string{"off", "egress", "egressing"}

// isOffTargetReport is true if the call reports the flight off the target.
func isOffTargetReport(tx string) bool {
	padded := " " + tx + " "
	for _, word := range offTargetWords {
		if strings.Contains(padded, " "+word+" ") {
			return true
		}
	}
	return false
}
