package orchestrator

import (
	"errors"
	"testing"
	"time"
)

func TestWithRetry(t *testing.T) {
	t.Run("Success on first attempt", func(t *testing.T) {
		attempts := 0
		err := WithRetry(func() error {
			attempts++
			return nil
		}, 3, 1, 10)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("Success after retries", func(t *testing.T) {
		attempts := 0
		err := WithRetry(func() error {
			attempts++
			if attempts < 3 {
				return errors.New("fail")
			}
			return nil
		}, 3, 1, 10)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("All retries fail", func(t *testing.T) {
		attempts := 0
		err := WithRetry(func() error {
			attempts++
			return errors.New("fail")
		}, 3, 1, 10)

		if err == nil {
			t.Error("expected error, got nil")
		}
		if attempts != 4 { // 0, 1, 2, 3 = 4 attempts
			t.Errorf("expected 4 attempts, got %d", attempts)
		}
	})
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		base    int
		max     int
		want    time.Duration
	}{
		{0, 100, 1000, 100 * time.Millisecond},
		{1, 100, 1000, 200 * time.Millisecond},
		{2, 100, 1000, 400 * time.Millisecond},
		{10, 100, 1000, 1000 * time.Millisecond}, // Capped
	}

	for _, tt := range tests {
		got := calculateBackoff(tt.attempt, tt.base, tt.max)
		if got != tt.want {
			t.Errorf("calculateBackoff(%d, %d, %d) = %v, want %v", tt.attempt, tt.base, tt.max, got, tt.want)
		}
	}
}
