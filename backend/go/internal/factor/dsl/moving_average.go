// Package dsl — operator implementations for the ALFQ factor DSL.
package dsl

import "math"

// Op is the interface for a DSL operator.
// Each operator maintains internal state for incremental evaluation.
type Op interface {
	Eval(v float64) float64
	Reset()
	Warmup() int
}

// moving average operators

type SMA struct {
	n     int
	buf   []float64
	idx   int
	sum   float64
	count int
}

func NewSMA(n int) *SMA { return &SMA{n: n, buf: make([]float64, n)} }

func (s *SMA) Warmup() int { return s.n }

func (s *SMA) Eval(v float64) float64 {
	old := s.buf[s.idx]
	s.buf[s.idx] = v
	s.idx = (s.idx + 1) % s.n
	s.sum += v - old
	if s.count < s.n {
		s.count++
	}
	if s.count < s.n {
		return math.NaN()
	}
	return s.sum / float64(s.n)
}

func (s *SMA) Reset() {
	s.sum = 0
	s.count = 0
	s.idx = 0
	for i := range s.buf {
		s.buf[i] = 0
	}
}

type EMA struct {
	n    int
	alpha float64
	value float64
	count int
}

func NewEMA(n int) *EMA { return &EMA{n: n, alpha: 2.0 / float64(n+1)} }

func (e *EMA) Warmup() int { return e.n }

func (e *EMA) Eval(v float64) float64 {
	if e.count == 0 {
		e.value = v
	} else {
		e.value = e.alpha*v + (1-e.alpha)*e.value
	}
	e.count++
	if e.count < e.n {
		return math.NaN()
	}
	return e.value
}

func (e *EMA) Reset() { e.value = 0; e.count = 0 }

type WMA struct {
	n     int
	buf   []float64
	idx   int
	count int
}

func NewWMA(n int) *WMA { return &WMA{n: n, buf: make([]float64, n)} }

func (w *WMA) Warmup() int { return w.n }

func (w *WMA) Eval(v float64) float64 {
	w.buf[w.idx] = v
	w.idx = (w.idx + 1) % w.n
	if w.count < w.n {
		w.count++
	}
	if w.count < w.n {
		return math.NaN()
	}
	var sum, weightSum float64
	for i := 0; i < w.n; i++ {
		weight := float64(i + 1)
		sum += w.buf[(w.idx+i)%w.n] * weight
		weightSum += weight
	}
	return sum / weightSum
}

func (w *WMA) Reset() {
	w.count = 0
	w.idx = 0
	for i := range w.buf {
		w.buf[i] = 0
	}
}
