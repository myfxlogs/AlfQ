package dsl

import "math"

// Ref returns value from n periods ago.
type Ref struct {
	n      int
	buf    []float64
	idx    int
	count  int
}

func NewRef(n int) *Ref { return &Ref{n: n, buf: make([]float64, n+1)} }

func (r *Ref) Warmup() int { return r.n }

func (r *Ref) Eval(v float64) float64 {
	r.buf[r.idx] = v
	r.idx = (r.idx + 1) % (r.n + 1)
	if r.count <= r.n {
		r.count++
	}
	if r.count <= r.n {
		return math.NaN()
	}
	return r.buf[(r.idx-r.n-1+r.n+1)%(r.n+1)]
}

func (r *Ref) Reset() {
	r.count = 0
	r.idx = 0
	for i := range r.buf {
		r.buf[i] = 0
	}
}

// Delta: v - ref(v, n)
type Delta struct {
	ref *Ref
	n   int
}

func NewDelta(n int) *Delta { return &Delta{ref: NewRef(n), n: n} }
func (d *Delta) Warmup() int { return d.n }
func (d *Delta) Eval(v float64) float64 {
	past := d.ref.Eval(v)
	if math.IsNaN(past) {
		return math.NaN()
	}
	return v - past
}
func (d *Delta) Reset() { d.ref.Reset() }

// PctChange: v / ref(v, n) - 1
type PctChange struct {
	ref *Ref
	n   int
}

func NewPctChange(n int) *PctChange { return &PctChange{ref: NewRef(n), n: n} }
func (p *PctChange) Warmup() int     { return p.n }
func (p *PctChange) Eval(v float64) float64 {
	past := p.ref.Eval(v)
	if math.IsNaN(past) || past == 0 {
		return math.NaN()
	}
	return v/past - 1
}
func (p *PctChange) Reset() { p.ref.Reset() }

// ZScore: (v - sma) / std
type ZScore struct {
	sma *SMA
	std *STD
}

func NewZScore(n int) *ZScore { return &ZScore{sma: NewSMA(n), std: NewSTD(n)} }
func (z *ZScore) Warmup() int { return nMax(z.sma.Warmup(), z.std.Warmup()) }
func (z *ZScore) Eval(v float64) float64 {
	ma := z.sma.Eval(v)
	sd := z.std.Eval(v)
	if math.IsNaN(ma) || math.IsNaN(sd) || sd == 0 {
		return math.NaN()
	}
	return (v - ma) / sd
}
func (z *ZScore) Reset() { z.sma.Reset(); z.std.Reset() }

// Rank: rolling percentile rank.
type Rank struct {
	n     int
	buf   []float64
	idx   int
	count int
}

func NewRank(n int) *Rank { return &Rank{n: n, buf: make([]float64, n)} }

func (r *Rank) Warmup() int { return r.n }

func (r *Rank) Eval(v float64) float64 {
	r.buf[r.idx] = v
	r.idx = (r.idx + 1) % r.n
	if r.count < r.n {
		r.count++
	}
	if r.count < r.n {
		return math.NaN()
	}
	// Count values <= v
	le := 0
	for i := 0; i < r.n; i++ {
		if r.buf[i] <= v {
			le++
		}
	}
	return float64(le) / float64(r.n)
}

func (r *Rank) Reset() {
	r.count = 0
	r.idx = 0
	for i := range r.buf {
		r.buf[i] = 0
	}
}

