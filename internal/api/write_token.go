package api

import (
	"crypto/subtle"
	"net/http"
)

// WriteTokenHeader carries the daemon's per-launch write token.
const WriteTokenHeader = "X-Mini-Orca-Token"

// RequireWriteToken admits a request that can change state (any method but
// GET, HEAD and OPTIONS) only when it carries this launch's write token. The
// Web UI receives the token in its document; other local clients read it from
// tokenFile, which only the daemon's user can read. Reads stay open so status
// probes and event streams need no token.
func RequireWriteToken(next http.Handler, token, tokenFile string) http.Handler {
	hint := "Reload the Web UI to get this daemon's write token."
	if tokenFile != "" {
		hint += " Other local clients send the token in " + WriteTokenHeader + "; it is in " + tokenFile + "."
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		sent := r.Header.Get(WriteTokenHeader)
		if token == "" || subtle.ConstantTimeCompare([]byte(sent), []byte(token)) != 1 {
			WriteAppError(w, NewAppError(ErrorUnauthorized, "missing or stale write token", hint, http.StatusUnauthorized, nil))
			return
		}
		next.ServeHTTP(w, r)
	})
}
