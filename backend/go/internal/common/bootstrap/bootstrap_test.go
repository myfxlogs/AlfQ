package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOptions(t *testing.T) {
	cfg := &runCfg{}
	WithoutPG()(cfg)
	if !cfg.skipPG {
		t.Fatal("expected skipPG")
	}
	WithoutRedis()(cfg)
	if !cfg.skipRDB {
		t.Fatal("expected skipRDB")
	}
	WithoutNATS()(cfg)
	if !cfg.skipNATS {
		t.Fatal("expected skipNATS")
	}
	WithoutCH()(cfg)
	if !cfg.skipCH {
		t.Fatal("expected skipCH")
	}
}

func TestHealthEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	registerHealthEndpoints(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{"/healthz", 200, "ok"},
		{"/metrics", 200, ""}, // promhttp returns metrics text, just check 200
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := http.Get(srv.URL + tt.path)
			if err != nil {
				t.Fatalf("%s: %v", tt.path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("%s: status %d, want %d", tt.path, resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestServeMuxAdapterHandle(t *testing.T) {
	mux := http.NewServeMux()
	adapter := &ServeMuxAdapter{Mux: mux}
	adapter.Mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestNewServer(t *testing.T) {
	mux := http.NewServeMux()
	srv := newServer(":0", mux)
	if srv == nil {
		t.Fatal("nil server")
	}
	if srv.Addr != ":0" {
		t.Fatalf("addr: %s", srv.Addr)
	}
}

func TestRunMinimal(t *testing.T) {
	// Test a minimal bootstrap with no PG/Redis, using a registrar that returns immediately.
	registered := make(chan struct{})
	register := func(mux *ServeMuxAdapter, d *Deps) error {
		close(registered)
		return nil
	}

	go func() {
		_ = Run("test-svc", register, WithoutPG(), WithoutRedis(), WithoutNATS(), WithoutCH())
	}()

	select {
	case <-registered:
		// registrar called, ok
	case <-time.After(2 * time.Second):
		t.Fatal("registrar was not called within timeout")
	}
}
