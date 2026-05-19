package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddleware(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	handler := CORSMiddleware(mux)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Test normal request
	resp, err := http.Get(srv.URL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("missing CORS header")
	}

	// Test OPTIONS preflight
	req, _ := http.NewRequest("OPTIONS", srv.URL+"/api", nil)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != 204 {
		t.Fatalf("OPTIONS status %d, want 204", resp2.StatusCode)
	}

	// Test with explicit Origin
	req3, _ := http.NewRequest("GET", srv.URL+"/api", nil)
	req3.Header.Set("Origin", "https://example.com")
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	origin := resp3.Header.Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Fatalf("expected Origin echo, got %q", origin)
	}
}

func TestDepsDefaults(t *testing.T) {
	d := &Deps{}
	if d.Log != nil {
		t.Fatal("Log should be nil by default")
	}
	if d.PG != nil {
		t.Fatal("PG should be nil by default")
	}
	if d.RDB != nil {
		t.Fatal("RDB should be nil by default")
	}
}

func TestServeMuxAdapterNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil mux")
		}
	}()
	a := &ServeMuxAdapter{Mux: nil}
	a.Mux.HandleFunc("/x", func(w http.ResponseWriter, r *http.Request) {})
}

func TestOptionsCompose(t *testing.T) {
	cfg := &runCfg{}
	WithoutPG()(cfg)
	WithoutRedis()(cfg)
	WithoutNATS()(cfg)
	if !cfg.skipPG || !cfg.skipRDB || !cfg.skipNATS {
		t.Fatal("all options should be applied")
	}
	if cfg.skipCH {
		t.Fatal("skipCH should not be set")
	}
}

func TestShutdownWithTimeout(t *testing.T) {
	mux := http.NewServeMux()
	srv := newServer(":0", mux)
	if srv == nil {
		t.Fatal("nil server")
	}
	// shutdownWithTimeout should handle nil/closed server gracefully
	shutdownWithTimeout(srv, 0)
}
