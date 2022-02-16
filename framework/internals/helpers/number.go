// Copyright The RAI Inc.
// The RAI Authors
package helpers

// TODO: Change to generic after go1.18 release
func FloatInRange(i, min, max float64) float64 {
	switch {
	case i < min:
		return min
	case i > max:
		return max
	default:
		return i
	}
}
