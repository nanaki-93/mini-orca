package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

var ErrBudgetExceeded = errors.New("agent run budget exceeded")

type BudgetError struct{ Limit string }

func (e *BudgetError) Error() string { return "agent run reached its " + e.Limit + " limit" }
func (e *BudgetError) Unwrap() error { return ErrBudgetExceeded }

// BudgetSnapshot contains no prompts, arguments, source, or credentials.
type BudgetSnapshot struct {
	ModelCalls           int   `json:"model_calls"`
	ToolCalls            int   `json:"tool_calls"`
	Repairs              int   `json:"repairs"`
	InputTokens          int64 `json:"input_tokens"`
	OutputTokens         int64 `json:"output_tokens"`
	UnknownUsageCalls    int   `json:"unknown_usage_calls"`
	ReservedInputTokens  int64 `json:"reserved_input_tokens"`
	ReservedOutputTokens int64 `json:"reserved_output_tokens"`
	ActiveCalls          int   `json:"active_calls"`
	ElapsedMillis        int64 `json:"elapsed_millis"`
}

type Budget struct {
	mu      sync.Mutex
	limits  config.AgentLimits
	usage   BudgetSnapshot
	started time.Time
	slots   chan struct{}
	cancel  context.CancelCauseFunc
}

type budgetKey struct{}

func BudgetFromContext(ctx context.Context) *Budget {
	budget, _ := ctx.Value(budgetKey{}).(*Budget)
	return budget
}

// WithBudget inherits an existing run rather than replenishing child/retry limits.
// The returned cancellation function owns only a newly created run.
func WithBudget(ctx context.Context, limits config.AgentLimits) (context.Context, context.CancelFunc, error) {
	if BudgetFromContext(ctx) != nil {
		return ctx, func() {}, nil
	}
	if err := limits.Validate(); err != nil {
		return nil, nil, err
	}
	owned, cancelCause := context.WithCancelCause(ctx)
	timed, stop := context.WithTimeoutCause(owned, time.Duration(limits.ElapsedSeconds)*time.Second, &BudgetError{Limit: "elapsed time"})
	budget := &Budget{limits: limits, started: time.Now(), slots: make(chan struct{}, limits.ConcurrentCalls), cancel: cancelCause}
	return context.WithValue(timed, budgetKey{}, budget), func() { stop(); cancelCause(context.Canceled) }, nil
}

func (b *Budget) Snapshot() BudgetSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	copy := b.usage
	copy.ElapsedMillis = time.Since(b.started).Milliseconds()
	return copy
}

type modelReservation struct {
	budget        *Budget
	input, output int64
	finished      bool // guarded by budget.mu
}

func (b *Budget) reserveModel(ctx context.Context, input, output int64) (*modelReservation, error) {
	if input < 0 || output < 0 {
		return nil, fmt.Errorf("invalid model token reservation")
	}
	select {
	case b.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	}
	b.mu.Lock()
	err := b.admitModelLocked(ctx, input, output)
	if err == nil {
		b.usage.ModelCalls++
		b.usage.ActiveCalls++
		b.usage.ReservedInputTokens += input
		b.usage.ReservedOutputTokens += output
	}
	b.mu.Unlock()
	if err != nil {
		<-b.slots
		if errors.Is(err, ErrBudgetExceeded) {
			b.cancel(err)
		}
		return nil, err
	}
	return &modelReservation{budget: b, input: input, output: output}, nil
}

func (b *Budget) admitModelLocked(ctx context.Context, input, output int64) error {
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	if b.usage.ModelCalls >= b.limits.ModelCalls {
		return &BudgetError{Limit: "model calls"}
	}
	if input > b.limits.InputTokens-b.usage.InputTokens-b.usage.ReservedInputTokens {
		return &BudgetError{Limit: "input tokens"}
	}
	if output > b.limits.OutputTokens-b.usage.OutputTokens-b.usage.ReservedOutputTokens {
		return &BudgetError{Limit: "output tokens"}
	}
	return nil
}

// settle returns unused reservations. Failed or interrupted requests retain
// their whole reservation because unreported upstream usage cannot be recovered.
func (r *modelReservation) settle(input, output int64, known, completed bool) error {
	if r == nil {
		return nil
	}
	b := r.budget
	b.mu.Lock()
	if r.finished {
		b.mu.Unlock()
		return nil
	}
	r.finished = true
	if !completed {
		input, output, known = r.input, r.output, false
	}
	b.usage.ReservedInputTokens -= r.input
	b.usage.ReservedOutputTokens -= r.output
	b.usage.ActiveCalls--
	if !known {
		b.usage.UnknownUsageCalls++
	}
	err := b.recordUsageLocked(input, output)
	b.mu.Unlock()
	<-b.slots
	if err != nil {
		b.cancel(err)
	}
	return err
}

func (b *Budget) recordUsageLocked(input, output int64) error {
	if input < 0 || output < 0 || input > 1<<50 || output > 1<<50 {
		return fmt.Errorf("%w: invalid provider usage", ErrBudgetExceeded)
	}
	b.usage.InputTokens += input
	b.usage.OutputTokens += output
	if b.usage.InputTokens+b.usage.ReservedInputTokens > b.limits.InputTokens {
		return &BudgetError{Limit: "input tokens"}
	}
	if b.usage.OutputTokens+b.usage.ReservedOutputTokens > b.limits.OutputTokens {
		return &BudgetError{Limit: "output tokens"}
	}
	return nil
}

func (b *Budget) AdmitTool(ctx context.Context) error {
	return b.admitCount(ctx, "tool calls", func() (*int, int) { return &b.usage.ToolCalls, b.limits.ToolCalls })
}

func (b *Budget) AdmitRepair(ctx context.Context) error {
	return b.admitCount(ctx, "repair attempts", func() (*int, int) { return &b.usage.Repairs, b.limits.Repairs })
}

func (b *Budget) admitCount(ctx context.Context, name string, counter func() (*int, int)) error {
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	b.mu.Lock()
	used, limit := counter()
	if *used >= limit {
		b.mu.Unlock()
		err := &BudgetError{Limit: name}
		b.cancel(err)
		return err
	}
	*used = *used + 1
	b.mu.Unlock()
	return nil
}
