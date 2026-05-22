// Package risksvc — AccountState computation helpers.
package risksvc

import "time"

// dailyPnLSnapshots tracks daily PnL per account.
type dailyPnLSnapshots map[string]*daySnapshot

type daySnapshot struct {
	date       string // YYYY-MM-DD
	startEquity float64
	currentPnL  float64
}

// ComputeDailyPnL returns the current daily PnL for an account.
// startEquity is the equity at the start of the current trading day.
// currentEquity is the latest equity from the broker stream.
func ComputeDailyPnL(startEquity, currentEquity float64) float64 {
	return currentEquity - startEquity
}

// ComputeMaxDrawdown returns the max drawdown ratio from peak equity.
// peakEquity is the all-time peak.
// currentEquity is the latest equity.
func ComputeMaxDrawdown(peakEquity, currentEquity float64) float64 {
	if peakEquity <= 0 {
		return 0
	}
	dd := (peakEquity - currentEquity) / peakEquity
	if dd < 0 {
		return 0
	}
	return dd
}

// SnapshotForDay returns the start-of-day equity for daily PnL tracking.
// On a new day, the snapshot date is updated and startEquity reset.
func SnapshotForDay(snap *daySnapshot, currentEquity float64) *daySnapshot {
	today := time.Now().UTC().Format("2006-01-02")
	if snap == nil || snap.date != today {
		return &daySnapshot{
			date:       today,
			startEquity: currentEquity,
			currentPnL:  0,
		}
	}
	snap.currentPnL = ComputeDailyPnL(snap.startEquity, currentEquity)
	return snap
}
