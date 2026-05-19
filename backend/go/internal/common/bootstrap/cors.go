// Package bootstrap — CORS middleware shared across all services.
package bootstrap

import "net/http"

// CORSMiddleware wraps an http.Handler with permissive CORS headers for ALFQ.
// Sets Origin, Methods, Headers, Expose-Headers, and handles OPTIONS preflight.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID, Connect-Protocol-Version, X-User-Agent")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Encoding, Grpc-Status, Grpc-Message, Connect-Protocol-Version")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
