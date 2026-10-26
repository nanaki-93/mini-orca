package client

import "context"

// IOGate is notified around blocking I/O so a caller that owns a lock can let
// other work (such as rendering the terminal UI) proceed while a request waits
// on the network. BeginIO and EndIO are always called in pairs and may be
// called concurrently from several goroutines.
type IOGate interface {
	BeginIO()
	EndIO()
}

type ioGateKey struct{}

// WithIOGate returns a context whose daemon requests report blocking I/O to gate.
func WithIOGate(ctx context.Context, gate IOGate) context.Context {
	if gate == nil {
		return ctx
	}
	return context.WithValue(ctx, ioGateKey{}, gate)
}

// EnterIO marks the start of blocking I/O for the gate carried by ctx, if any,
// and returns the function that marks its end.
func EnterIO(ctx context.Context) (exit func()) {
	gate, _ := ctx.Value(ioGateKey{}).(IOGate)
	if gate == nil {
		return func() {}
	}
	gate.BeginIO()
	return gate.EndIO
}
