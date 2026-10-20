package tui

import "github.com/nanaki-93/mini-orca/v2/internal/tui/client"

// Response holds the HTTP status code and raw response body from the daemon.
type Response = client.Response

// Client communicates with the local Mini-Orca daemon over guarded HTTP routes.
type Client = client.Client

// NewClient validates the daemon origin URL and constructs a guarded Client.
func NewClient(base string) (*Client, error) {
	return client.NewClient(base)
}
