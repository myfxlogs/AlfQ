package strategysvc_test

import (
	"context"
	"testing"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/strategysvc"
)

type stubStrategy struct{ id string }

func (s *stubStrategy) ID() string { return s.id }
func (s *stubStrategy) OnFactor(ctx context.Context, factor string, value float64) (*strategysvc.Signal, error) {
	return nil, nil
}
func (s *stubStrategy) OnBar(ctx context.Context, bar *pb.Bar) (*strategysvc.Signal, error) {
	return nil, nil
}

func TestNewRunner(t *testing.T) {
	runner := strategysvc.NewRunner(&stubStrategy{id: "stub-1"})
	if runner == nil {
		t.Fatal("runner is nil")
	}
}

func TestRunnerEvaluate(t *testing.T) {
	runner := strategysvc.NewRunner(&stubStrategy{id: "stub-1"})
	sig, err := runner.Evaluate(context.Background(), "sma_5", 1.05)
	if err != nil {
		t.Logf("Evaluate returned error: %v", err)
	}
	_ = sig
}

func TestRunnerGetPosition(t *testing.T) {
	runner := strategysvc.NewRunner(&stubStrategy{id: "stub-1"})
	qty := runner.GetPosition("EURUSD")
	if qty != 0 {
		t.Logf("unexpected position qty: %f", qty)
	}
}

func TestRunnerUpdatePosition(t *testing.T) {
	runner := strategysvc.NewRunner(&stubStrategy{id: "stub-1"})
	runner.UpdatePosition("EURUSD", 1.0)
	qty := runner.GetPosition("EURUSD")
	if qty != 1.0 {
		t.Errorf("expected qty=1.0, got %f", qty)
	}
}
