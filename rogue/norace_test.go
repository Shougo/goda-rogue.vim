//go:build !race

package rogue

// raceDetector says whether the tests run with the race detector, which
// makes them several times slower.
const raceDetector = false
