// Package risksvc — additional risk rules (M4).
package risksvc

import (
	"context"
	"fmt"
	"math"
	"time"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Session rejects orders outside allowed trading hours.
type Session struct {
	timezone *time.Location
}

func NewSession(tz string) *Session {
	loc, _ := time.LoadLocation(tz)
	if loc == nil {
		loc = time.UTC
	}
	return &Session{timezone: loc}
}

func (r *Session) Name() string { return "session" }

func (r *Session) Check(_ context.Context, req *pb.OrderRequest, _ *AccountState) *pb.RiskCheckResult {
	now := time.Now().In(r.timezone)
	day := now.Weekday()
	hour := now.Hour()

	// Simplified: reject weekends
	if day == time.Saturday || day == time.Sunday {
		return &pb.RiskCheckResult{Approved: false, Reason: "market closed (weekend)", RuleId: r.Name()}
	}
	// Reject outside typical forex hours (Mon 00:00 - Fri 22:00)
	if hour < 1 && day == time.Monday {
		return &pb.RiskCheckResult{Approved: false, Reason: "market not yet open", RuleId: r.Name()}
	}
	return &pb.RiskCheckResult{Approved: true}
}

// Margin rejects orders that would exceed available margin.
type Margin struct{ minMarginLevel float64 }

func NewMargin(minMarginLevel float64) *Margin {
	if minMarginLevel == 0 {
		minMarginLevel = 1.5 // 150% margin level required
	}
	return &Margin{minMarginLevel: minMarginLevel}
}

func (r *Margin) Name() string { return "margin" }

func (r *Margin) Check(_ context.Context, req *pb.OrderRequest, state *AccountState) *pb.RiskCheckResult {
	if state.Equity <= 0 || state.Margin <= 0 {
		return &pb.RiskCheckResult{Approved: true} // no position yet
	}
	marginLevel := state.Equity / state.Margin
	if marginLevel < r.minMarginLevel {
		return &pb.RiskCheckResult{
			Approved: false,
			Reason:   fmt.Sprintf("margin level %.2f below minimum %.2f", marginLevel, r.minMarginLevel),
			RuleId:   r.Name(),
		}
	}
	return &pb.RiskCheckResult{Approved: true}
}

// Slippage rejects if recent slippage exceeds threshold.
type Slippage struct {
	maxSlippagePoints float64
	recentFills       []slippageRecord
}

type slippageRecord struct {
	symbol   string
	slippage float64
	ts       time.Time
}

func NewSlippage(maxPoints float64) *Slippage {
	return &Slippage{maxSlippagePoints: maxPoints}
}

func (r *Slippage) Name() string { return "slippage" }

func (r *Slippage) Check(_ context.Context, req *pb.OrderRequest, _ *AccountState) *pb.RiskCheckResult {
	// Check recent slippage records for the same symbol
	cutoff := time.Now().Add(-5 * time.Minute)
	recent := 0
	totalSlip := 0.0
	for _, rec := range r.recentFills {
		if rec.symbol == req.Symbol && rec.ts.After(cutoff) {
			recent++
			totalSlip += math.Abs(rec.slippage)
		}
	}
	if recent >= 3 && totalSlip/float64(recent) > r.maxSlippagePoints {
		return &pb.RiskCheckResult{
			Approved: false,
			Reason:   fmt.Sprintf("avg slippage %.2f pts exceeds limit %.2f", totalSlip/float64(recent), r.maxSlippagePoints),
			RuleId:   r.Name(),
		}
	}
	return &pb.RiskCheckResult{Approved: true}
}

// RecordSlippage records a fill slippage for future checks.
func (r *Slippage) RecordSlippage(symbol string, slipPoints float64) {
	r.recentFills = append(r.recentFills, slippageRecord{symbol: symbol, slippage: slipPoints, ts: time.Now()})
	// Keep only last 100 records
	if len(r.recentFills) > 100 {
		r.recentFills = r.recentFills[len(r.recentFills)-100:]
	}
}

// Heartbeat rejects if the broker connection is deemed stale.
type Heartbeat struct {
	lastHeartbeat time.Time
	timeout       time.Duration
}

func NewHeartbeat(timeout time.Duration) *Heartbeat {
	return &Heartbeat{timeout: timeout, lastHeartbeat: time.Now()}
}

func (r *Heartbeat) Name() string { return "heartbeat" }

func (r *Heartbeat) Check(_ context.Context, req *pb.OrderRequest, _ *AccountState) *pb.RiskCheckResult {
	if time.Since(r.lastHeartbeat) > r.timeout {
		return &pb.RiskCheckResult{
			Approved: false,
			Reason:   fmt.Sprintf("broker heartbeat lost for %v", time.Since(r.lastHeartbeat).Round(time.Second)),
			RuleId:   r.Name(),
		}
	}
	return &pb.RiskCheckResult{Approved: true}
}

// UpdateHeartbeat records a heartbeat event.
func (r *Heartbeat) UpdateHeartbeat() {
	r.lastHeartbeat = time.Now()
}

// RejectRate rejects if too many recent orders have been rejected.
type RejectRate struct {
	maxRejectRate float64
	recentTotal   int
	recentRejects int
	window        time.Duration
	windowStart   time.Time
}

func NewRejectRate(maxRate float64, window time.Duration) *RejectRate {
	return &RejectRate{maxRejectRate: maxRate, window: window, windowStart: time.Now()}
}

func (r *RejectRate) Name() string { return "reject_rate" }

func (r *RejectRate) Check(_ context.Context, req *pb.OrderRequest, _ *AccountState) *pb.RiskCheckResult {
	if time.Since(r.windowStart) > r.window {
		r.windowStart = time.Now()
		r.recentTotal = 0
		r.recentRejects = 0
	}
	r.recentTotal++
	if r.recentTotal > 10 {
		rate := float64(r.recentRejects) / float64(r.recentTotal)
		if rate > r.maxRejectRate {
			return &pb.RiskCheckResult{
				Approved: false,
				Reason:   fmt.Sprintf("reject rate %.2f exceeds limit %.2f", rate, r.maxRejectRate),
				RuleId:   r.Name(),
			}
		}
	}
	return &pb.RiskCheckResult{Approved: true}
}

// RecordReject records a rejection for rate tracking.
func (r *RejectRate) RecordReject() {
	r.recentRejects++
}
