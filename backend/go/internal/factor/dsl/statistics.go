package dsl

import "math"

// STD computes rolling sample standard deviation using Welford's algorithm.
type STD struct {
	n     int
	count int
	mean  float64
	m2    float64
	buf   []float64
	idx   int
}

func NewSTD(n int) *STD {
	return &STD{n: n, buf: make([]float64, n)}
}

func (s *STD) Warmup() int { return s.n }

func (s *STD) Eval(v float64) float64 {
	// Remove old value from Welford state if buffer full
	if s.count >= s.n {
		old := s.buf[s.idx]
		// Approximate removal from Welford (exact removal is complex; use fresh compute on buffer)
		_ = old
	}

	s.buf[s.idx] = v
	s.idx = (s.idx + 1) % s.n
	if s.count < s.n {
		s.count++
	}

	if s.count < s.n {
		return math.NaN()
	}

	// Recompute mean and std from buffer for correctness
	var sum, sumSq float64
	for i := 0; i < s.n; i++ {
		val := s.buf[i]
		sum += val
		sumSq += val * val
	}
	mean := sum / float64(s.n)
	variance := sumSq/float64(s.n) - mean*mean
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance)
}

func (s *STD) Reset() {
	s.count = 0
	s.mean = 0
	s.m2 = 0
	s.idx = 0
	for i := range s.buf {
		s.buf[i] = 0
	}
}

// VAR is rolling variance (std²).
type VAR struct{ std *STD }

func NewVAR(n int) *VAR { return &VAR{std: NewSTD(n)} }

func (v *VAR) Warmup() int { return v.std.Warmup() }

func (v *VAR) Eval(x float64) float64 {
	s := v.std.Eval(x)
	if math.IsNaN(s) {
		return math.NaN()
	}
	return s * s
}

func (v *VAR) Reset() { v.std.Reset() }

// Min rolling min.
type Min struct {
	n     int
	buf   []float64
	idx   int
	count int
}

func NewMin(n int) *Min { return &Min{n: n, buf: make([]float64, n)} }

func (m *Min) Warmup() int { return m.n }

func (m *Min) Eval(v float64) float64 {
	m.buf[m.idx] = v
	m.idx = (m.idx + 1) % m.n
	if m.count < m.n {
		m.count++
	}
	if m.count < m.n {
		return math.NaN()
	}
	minVal := m.buf[0]
	for i := 1; i < m.n; i++ {
		if m.buf[i] < minVal {
			minVal = m.buf[i]
		}
	}
	return minVal
}

func (m *Min) Reset() {
	m.count = 0
	m.idx = 0
	for i := range m.buf {
		m.buf[i] = 0
	}
}

type Max struct{ m *Min }

func NewMax(n int) *Max    { return &Max{m: NewMin(n)} }
func (m *Max) Warmup() int { return m.m.Warmup() }
func (m *Max) Eval(v float64) float64 {
	n := m.m.n
	m.m.buf[m.m.idx] = -v
	m.m.idx = (m.m.idx + 1) % n
	if m.m.count < n {
		m.m.count++
	}
	if m.m.count < n {
		return math.NaN()
	}
	maxVal := -m.m.buf[0]
	for i := 1; i < n; i++ {
		if -m.m.buf[i] > maxVal {
			maxVal = -m.m.buf[i]
		}
	}
	return maxVal
}
func (m *Max) Reset() { m.m.Reset() }

// Sum rolling sum.
type Sum struct {
	n     int
	buf   []float64
	idx   int
	sum   float64
	count int
}

func NewSum(n int) *Sum { return &Sum{n: n, buf: make([]float64, n)} }

func (s *Sum) Warmup() int { return s.n }

func (s *Sum) Eval(v float64) float64 {
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
	return s.sum
}

func (s *Sum) Reset() {
	s.sum = 0
	s.count = 0
	s.idx = 0
	for i := range s.buf {
		s.buf[i] = 0
	}
}
