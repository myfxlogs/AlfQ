package factorsvc

import (
	"testing"
)

func TestMetrics_NotNil(t *testing.T) {
	if FactorEvalTotal == nil {
		t.Fatal("FactorEvalTotal should not be nil")
	}
	if FactorEvalDuration == nil {
		t.Fatal("FactorEvalDuration should not be nil")
	}
	if FactorLoadedCount == nil {
		t.Fatal("FactorLoadedCount should not be nil")
	}
	if FactorDependencyDepth == nil {
		t.Fatal("FactorDependencyDepth should not be nil")
	}
}
