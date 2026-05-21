package adminapi

import (
	"testing"
)

func TestServiceConfig_Length(t *testing.T) {
	if len(serviceConfig) == 0 {
		t.Fatal("serviceConfig should not be empty")
	}
}

func TestServiceConfig_RequiredServices(t *testing.T) {
	required := []string{"trading-core", "md-gateway", "quant-engine"}
	for _, req := range required {
		found := false
		for _, sc := range serviceConfig {
			if sc.Name == req {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("required service %s not found in config", req)
		}
	}
}
