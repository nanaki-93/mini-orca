package api

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// LocalOnly protects the daemon's native-client API boundary before route
// handlers can observe a request. External hosts are accepted only when the
// daemon was deliberately bound beyond loopback; that does not authenticate
// those requests.
func LocalOnly(next http.Handler, allowExternalHosts bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowExternalHosts && !isLoopbackHost(r.Host) {
			WriteAppError(w, Forbidden("unexpected Host header", "Use the local Mini-Orca daemon address.", nil))
			return
		}
		if isCORSPreflight(r) {
			WriteAppError(w, Forbidden("browser preflight requests are not supported", "Use the local Mini-Orca client.", nil))
			return
		}
		if r.Header.Get("Origin") != "" && !isAllowedWebUIOrigin(r, allowExternalHosts) && !isSameOriginWebUIAsset(r, allowExternalHosts) {
			WriteAppError(w, Forbidden("browser Origin headers are not supported", "Use the local Mini-Orca client.", nil))
			return
		}
		if requiresJSON(r.Method) && requestHasBody(r) && !isJSONContentType(r.Header.Get("Content-Type")) {
			WriteAppError(w, NewAppError(ErrorBadRequest, "application/json Content-Type is required", "Send this request as JSON.", http.StatusUnsupportedMediaType, nil))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAllowedWebUIOrigin(r *http.Request, allowExternalHosts bool) bool {
	if r.Header.Get("X-Mini-Orca-Web-UI") != "1" {
		return false
	}
	return isSameDaemonOrigin(r, allowExternalHosts)
}

// isSameOriginWebUIAsset admits the Origin header browsers attach to ES module
// script fetches. Only side-effect-free GET/HEAD requests for the embedded Web UI
// shell and its static assets qualify, and only from the daemon's own origin.
func isSameOriginWebUIAsset(r *http.Request, allowExternalHosts bool) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if r.URL.Path != "/ui" && !strings.HasPrefix(r.URL.Path, "/ui/assets/") {
		return false
	}
	return isSameDaemonOrigin(r, allowExternalHosts)
}

func isSameDaemonOrigin(r *http.Request, allowExternalHosts bool) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin == nil || origin.Scheme != "http" || origin.Host == "" {
		return false
	}
	if !strings.EqualFold(origin.Host, r.Host) {
		return false
	}
	return allowExternalHosts || isLoopbackHost(origin.Host)
}

func isLoopbackHost(host string) bool {
	hostname := host
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		hostname = parsedHost
	}
	hostname = strings.Trim(hostname, "[]")
	if strings.EqualFold(hostname, "localhost") {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}

func isCORSPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}

func requiresJSON(method string) bool {
	return method == http.MethodPost || method == http.MethodPatch
}

func requestHasBody(r *http.Request) bool {
	return r.Body != nil && r.ContentLength != 0
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "application/json"
}
