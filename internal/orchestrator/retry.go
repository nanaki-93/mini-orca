package orchestrator

import (
	"fmt"
	"math"
	"time"
)

// WithRetry executes fn with exponential backoff retry.
// It retries up to maxRetries times with delay = min(backoffBase * 2^attempt, backoffMax).
func WithRetry(fn func() error, maxRetries int, backoffBase, backoffMax int) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err
		if attempt == maxRetries {
			break
		}

		delay := time.Duration(math.Min(
			float64(backoffBase)*math.Pow(2, float64(attempt)),
			float64(backoffMax),
		)) * time.Millisecond

		fmt.Printf("[Retry] Attempt %d/%d failed: %v. Retrying in %v...\n",
			attempt+1, maxRetries, err, delay)
		time.Sleep(delay)
	}

	return fmt.Errorf("all %d retries exhausted: %w", maxRetries, lastErr)
}
