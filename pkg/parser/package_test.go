package parser

import (
	"testing"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/stretchr/testify/require"
)

func newPackageParser() *Parser {
	p := New(TestCallsign, []string{}, true)
	p.SetPackageControllers([]string{"Darkstar"})
	return p
}

func TestParserPackageRequests(t *testing.T) {
	t.Parallel()
	testCases := []parserTestCase{
		{
			text:     "Darkstar, Colt 1-1, checking in.",
			expected: &brevity.PackageRequest{Callsign: "colt 1 1", Controller: "darkstar", Kind: brevity.PackageCheckIn},
		},
		{
			text:     "Darkstar, Colt 11, check in",
			expected: &brevity.PackageRequest{Callsign: "colt 1 1", Controller: "darkstar", Kind: brevity.PackageCheckIn},
		},
		{
			text:     "Darkstar, Colt 1-1, ready to push.",
			expected: &brevity.PackageRequest{Callsign: "colt 1 1", Controller: "darkstar", Kind: brevity.PackageReadyToPush},
		},
		{
			text:     "Darkstar, Colt 1-1, say push time.",
			expected: &brevity.PackageRequest{Callsign: "colt 1 1", Controller: "darkstar", Kind: brevity.PackagePushTime},
		},
		{
			text:     "Darkstar Colt 1 1 push time",
			expected: &brevity.PackageRequest{Callsign: "colt 1 1", Controller: "darkstar", Kind: brevity.PackagePushTime},
		},
		{
			text:     "Skyeye, Colt 1-1, ready to push.",
			expected: &brevity.PackageRequest{Callsign: "colt 1 1", Controller: "skyeye", Kind: brevity.PackageReadyToPush},
		},
	}
	runParserTestCases(t, newPackageParser(), testCases, func(t *testing.T, test parserTestCase, request any) {
		t.Helper()
		expected := test.expected.(*brevity.PackageRequest)
		actual := request.(*brevity.PackageRequest)
		require.Equal(t, *expected, *actual)
	})
}

func TestParserCheckInToTheGCIStaysACheckIn(t *testing.T) {
	t.Parallel()
	request := newPackageParser().Parse("Skyeye, Colt 1-1, checking in.")
	require.IsType(t, &brevity.CheckInRequest{}, request)
}

func TestParserGCIRequestsAreUnchangedWithPackageControllers(t *testing.T) {
	t.Parallel()
	request := newPackageParser().Parse("Darkstar, Colt 1-1, picture.")
	require.IsType(t, &brevity.PictureRequest{}, request)
	request = newPackageParser().Parse("Skyeye, Colt 1-1, radio check.")
	require.IsType(t, &brevity.RadioCheckRequest{}, request)
}

func TestParserIgnoresAnUnknownControllerCallsign(t *testing.T) {
	t.Parallel()
	require.Nil(t, New(TestCallsign, []string{}, true).Parse("Darkstar, Colt 1-1, checking in."))
}

func TestParserTakesANewCallsign(t *testing.T) {
	t.Parallel()
	p := newPackageParser()
	p.SetCallsign("Chainsaw")
	require.IsType(t, &brevity.RadioCheckRequest{}, p.Parse("Chainsaw, Colt 1-1, radio check."))
	require.Nil(t, p.Parse("Skyeye, Colt 1-1, radio check."))
}

func TestParseLoosePackageCall(t *testing.T) {
	t.Parallel()
	p := newPackageParser()
	require.Equal(t,
		&brevity.PackageRequest{Callsign: "colt 1 1", Controller: "darkstar", Kind: brevity.PackageCheckIn},
		p.ParseLoosePackageCall("Colt 1-1, checking in."))
	require.Equal(t,
		&brevity.PackageRequest{Callsign: "colt 1 1", Controller: "darkstar", Kind: brevity.PackageReadyToPush},
		p.ParseLoosePackageCall("Colt 1-1, ready to push."))
	require.Equal(t,
		&brevity.PackageRequest{Callsign: "colt 1 1", Controller: "darkstar", Kind: brevity.PackagePushTime},
		p.ParseLoosePackageCall("Colt 1 1 say push time"))
	require.Nil(t, p.ParseLoosePackageCall("nice shot Colt"))
	require.Nil(t, New(TestCallsign, []string{}, true).ParseLoosePackageCall("Colt 1-1, ready to push."))
}
