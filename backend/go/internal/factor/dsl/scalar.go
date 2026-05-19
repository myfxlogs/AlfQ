package dsl

import "math"

// Scalar operators (stateless).

func Abs(x float64) float64 { return math.Abs(x) }
func Sign(x float64) float64 {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}
func Log(x float64) float64 {
	if x <= 0 {
		return math.NaN()
	}
	return math.Log(x)
}
func Exp(x float64) float64 { return math.Exp(x) }
func Sqrt(x float64) float64 {
	if x < 0 {
		return math.NaN()
	}
	return math.Sqrt(x)
}
func Pow(x, n float64) float64 { return math.Pow(x, n) }
