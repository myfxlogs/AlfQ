package dsl

import "math"

// Corr computes rolling Pearson correlation between two series.
type Corr struct {
	n     int
	xBuf  []float64
	yBuf  []float64
	idx   int
	count int
}

func NewCorr(n int) *Corr {
	return &Corr{n: n, xBuf: make([]float64, n), yBuf: make([]float64, n)}
}

func (c *Corr) Warmup() int { return c.n }

func (c *Corr) Eval(x, y float64) float64 {
	c.xBuf[c.idx] = x
	c.yBuf[c.idx] = y
	c.idx = (c.idx + 1) % c.n
	if c.count < c.n {
		c.count++
	}
	if c.count < c.n {
		return math.NaN()
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := 0; i < c.n; i++ {
		xi := c.xBuf[i]
		yi := c.yBuf[i]
		sumX += xi
		sumY += yi
		sumXY += xi * yi
		sumX2 += xi * xi
		sumY2 += yi * yi
	}
	num := float64(c.n)*sumXY - sumX*sumY
	den := math.Sqrt((float64(c.n)*sumX2 - sumX*sumX) * (float64(c.n)*sumY2 - sumY*sumY))
	if den == 0 {
		return math.NaN()
	}
	return num / den
}

func (c *Corr) Reset() {
	c.count = 0
	c.idx = 0
	for i := range c.xBuf {
		c.xBuf[i] = 0
		c.yBuf[i] = 0
	}
}

// Cov computes rolling covariance between two series.
type Cov struct {
	n     int
	xBuf  []float64
	yBuf  []float64
	idx   int
	count int
}

func NewCov(n int) *Cov {
	return &Cov{n: n, xBuf: make([]float64, n), yBuf: make([]float64, n)}
}

func (c *Cov) Warmup() int { return c.n }

func (c *Cov) Eval(x, y float64) float64 {
	c.xBuf[c.idx] = x
	c.yBuf[c.idx] = y
	c.idx = (c.idx + 1) % c.n
	if c.count < c.n {
		c.count++
	}
	if c.count < c.n {
		return math.NaN()
	}

	var sumX, sumY float64
	for i := 0; i < c.n; i++ {
		sumX += c.xBuf[i]
		sumY += c.yBuf[i]
	}
	meanX := sumX / float64(c.n)
	meanY := sumY / float64(c.n)

	var cov float64
	for i := 0; i < c.n; i++ {
		cov += (c.xBuf[i] - meanX) * (c.yBuf[i] - meanY)
	}
	return cov / float64(c.n)
}

func (c *Cov) Reset() {
	c.count = 0
	c.idx = 0
	for i := range c.xBuf {
		c.xBuf[i] = 0
		c.yBuf[i] = 0
	}
}
