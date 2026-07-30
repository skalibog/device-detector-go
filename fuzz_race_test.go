//go:build race

package devicedetector

// raceEnabled reports whether the race detector is active. Race
// instrumentation slows regexp2 backtracking 10-20x, so wall-time assertions
// measure the instrumentation, not the code under test.
const raceEnabled = true
