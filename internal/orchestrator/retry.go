package orchestrator

import (
	"fmt"
	"math"
	"time"
)

// WithRetry executes the given function with retry logic and exponential backoff.
// It retries up to maxRetries times, calculating the delay between attempts
// using exponential backoff: delay = min(backoffBase * 2^attempt, backoffMax).
// Logs each retry attempt and returns the final error after all retries are exhausted.
func WithRetry(fn func() error, maxRetries int, backoffBase, backoffMax int) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		// Store the last error
		lastErr = err

		// If we've exhausted all retries, return the error
		if attempt == maxRetries {
			break
		}

		// Calculate exponential backoff delay
		delay := calculateBackoff(attempt, backoffBase, backoffMax)

		// Log the retry attempt
		fmt.Printf("[Retry] Attempt %d/%d failed: %v. Retrying in %v...\n",
			attempt+1, maxRetries, err, delay)

		// Wait before next attempt
		time.Sleep(delay)
	}

	return fmt.Errorf("all %d retries exhausted: %w", maxRetries, lastErr)
}

// calculateBackoff calculates the delay for a given attempt using exponential backoff.
// The formula is: min(backoffBase * 2^attempt, backoffMax).
func calculateBackoff(attempt int, backoffBase, backoffMax int) time.Duration {
	// Calculate exponential delay: backoffBase * 2^attempt
	exponentialDelay := float64(backoffBase) * math.Pow(2, float64(attempt))

	// Cap at maximum delay
	delay := exponentialDelay
	if delay > float64(backoffMax) {
		delay = float64(backoffMax)
	}

	return time.Duration(delay) * time.Millisecond
}
