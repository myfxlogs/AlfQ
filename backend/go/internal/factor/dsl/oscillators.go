package dsl

import "math"

// RSI: Relative Strength Index (Wilder's smoothing).
type RSI struct {
	n       int
	avgGain float64
	avgLoss float64
	prev    float64
	count   int
}

func NewRSI(n int) *RSI { return &RSI{n: n} }

func (r *RSI) Warmup() int { return r.n + 1 }

func (r *RSI) Eval(v float64) float64 {
	if r.count == 0 {
		r.prev = v
		r.count++
		return math.NaN()
	}

	change := v - r.prev
	r.prev = v
	gain, loss := 0.0, 0.0
	if change > 0 {
		gain = change
	} else {
		loss = -change
	}

	if r.count < r.n+1 {
		r.avgGain += gain
		r.avgLoss += loss
		r.count++
		if r.count < r.n+1 {
			return math.NaN()
		}
		r.avgGain /= float64(r.n)
		r.avgLoss /= float64(r.n)
	} else {
		r.avgGain = (r.avgGain*float64(r.n-1) + gain) / float64(r.n)
		r.avgLoss = (r.avgLoss*float64(r.n-1) + loss) / float64(r.n)
		r.count++
	}

	if r.avgLoss == 0 {
		return 100.0
	}
	rs := r.avgGain / r.avgLoss
	return 100.0 - 100.0/(1.0+rs)
}

func (r *RSI) Reset() { r.avgGain = 0; r.avgLoss = 0; r.prev = 0; r.count = 0 }

// MACD line: ema(fast) - ema(slow).
type MACD struct {
	fastEMA *EMA
	slowEMA *EMA
}

func NewMACD(fast, slow int) *MACD {
	return &MACD{fastEMA: NewEMA(fast), slowEMA: NewEMA(slow)}
}

func (m *MACD) Warmup() int { return m.slowEMA.Warmup() }

func (m *MACD) Eval(v float64) float64 {
	f := m.fastEMA.Eval(v)
	s := m.slowEMA.Eval(v)
	if math.IsNaN(f) || math.IsNaN(s) {
		return math.NaN()
	}
	return f - s
}

func (m *MACD) Reset() { m.fastEMA.Reset(); m.slowEMA.Reset() }

// ATR: Average True Range (Wilder's smoothing).
type ATR struct {
	n       int
	tr      *EMA
	prev    float64
	count   int
	hasPrev bool
}

func NewATR(n int) *ATR { return &ATR{n: n, tr: NewEMA(n)} }

func (a *ATR) Warmup() int { return a.n + 1 }

func (a *ATR) Eval(v float64) float64 {
	if !a.hasPrev {
		a.hasPrev = true
		a.prev = v
		return math.NaN()
	}
	tr := math.Abs(v - a.prev)
	a.prev = v
	return a.tr.Eval(tr)
}

func (a *ATR) Reset() { a.tr.Reset(); a.hasPrev = false; a.count = 0 }
