package tui

import "github.com/nanaki-93/mini-orca/v2/internal/tui/tuiclient"

// Response holds the HTTP status code and raw response body from the daemon.
type Response = tuiclient.Response

// Client communicates with the local Mini-Orca daemon over guarded HTTP routes.
type Client = tuiclient.Client

// NewClient validates the daemon origin URL and constructs a guarded Client.
func NewClient(base string) (*Client, error) {
	return tuiclient.NewClient(base)
}
