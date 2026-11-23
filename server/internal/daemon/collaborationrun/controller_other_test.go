//go:build !linux

package collaborationrun

func runPreparedTestFixture() (int, bool) { return 0, false }

func runTestAgent() int { return 125 }
