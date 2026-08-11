// Package api provides HTTP handlers for the Mini-Orca REST API.
package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// ─── Cache Entry ──────────────────────────────────────────────────────────────

// cacheEntry stores a cached response with its timestamp and ETag.
type cacheEntry struct {
	body      []byte
	Etag      string
	createdAt time.Time
	ttl       time.Duration
}

// isExpired checks if the cache entry has expired.
func (e *cacheEntry) isExpired() bool {
	return time.Since(e.createdAt) > e.ttl
}

// ─── Response Cache ───────────────────────────────────────────────────────────

// ResponseCache provides simple in-memory caching with ETag support.
type ResponseCache struct {
	mu         sync.RWMutex
	entries    map[string]*cacheEntry
	defaultTTL time.Duration
}

// NewResponseCache creates a new response cache with the given default TTL.
func NewResponseCache(defaultTTL time.Duration) *ResponseCache {
	return &ResponseCache{
		entries:    make(map[string]*cacheEntry),
		defaultTTL: defaultTTL,
	}
}

// Get retrieves a cached entry by key.
func (c *ResponseCache) Get(key string) (*cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if entry.isExpired() {
		delete(c.entries, key)
		return nil, false
	}

	return entry, true
}

// Set stores a new cache entry.
func (c *ResponseCache) Set(key string, body []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	etag := computeETag(body)

	c.entries[key] = &cacheEntry{
		body:      body,
		Etag:      etag,
		createdAt: time.Now(),
		ttl:       ttl,
	}
}

// Delete removes a cache entry by key.
func (c *ResponseCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// Clear removes all cache entries.
func (c *ResponseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}

// computeETag generates an ETag hash for the given body.
func computeETag(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])[:16]
}

// ─── Cache Middleware ─────────────────────────────────────────────────────────

// cacheMiddleware returns an HTTP middleware that caches responses.
func cacheMiddleware(cache *ResponseCache, defaultTTL time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only cache GET requests
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			// Check if client sent an If-None-Match header
			ifNoneMatch := r.Header.Get("If-None-Match")
			if ifNoneMatch != "" {
				if entry, ok := cache.Get(r.URL.Path); ok && entry.Etag == ifNoneMatch[1:len(ifNoneMatch)-1] {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}

			// Wrap the response writer to capture the body
			cw := &captureWriter{
				ResponseWriter: w,
				body:           make([]byte, 0),
			}

			next.ServeHTTP(cw, r)

			// Only cache successful responses with appropriate content type
			if cw.status == http.StatusOK &&
				(cw.contentType == "text/html" || cw.contentType == "application/json") {
				cache.Set(r.URL.Path, cw.body, defaultTTL)

				// Add ETag header
				etag := computeETag(cw.body)
				w.Header().Set("ETag", "\""+etag+"\"")
				w.Header().Set("Cache-Control", "public, max-age=10")
			}
		})
	}
}

// captureWriter wraps http.ResponseWriter to capture the response body.
type captureWriter struct {
	http.ResponseWriter
	body        []byte
	status      int
	contentType string
}

// WriteHeader captures the status code.
func (cw *captureWriter) WriteHeader(status int) {
	cw.status = status
	cw.ResponseWriter.WriteHeader(status)
}

// Write captures the body.
func (cw *captureWriter) Write(b []byte) (int, error) {
	cw.body = append(cw.body, b...)
	return cw.ResponseWriter.Write(b)
}

// Header returns the headers.
func (cw *captureWriter) Header() http.Header {
	return cw.ResponseWriter.Header()
}

// ─── Cache Invalidation ───────────────────────────────────────────────────────

// cacheInvalidator provides cache invalidation for session-related data.
type cacheInvalidator struct {
	cache *ResponseCache
	mu    sync.Mutex
}

// newCacheInvalidator creates a new cache invalidator.
func newCacheInvalidator(cache *ResponseCache) *cacheInvalidator {
	return &cacheInvalidator{
		cache: cache,
	}
}

// invalidateAll invalidates all cached responses.
func (i *cacheInvalidator) invalidateAll() {
	i.cache.Clear()
}
