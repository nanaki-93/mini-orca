//go:build windows

package insighteval

// Windows support fails closed until it has an equivalent process-scoped lock.
// This keeps evaluation from dispatching without crash-safe serialization.
type runnerLockedFile struct{}

func lockRunnerFile(string) (*runnerLockedFile, error) { return nil, ErrEngineeringInsightRunLocked }
func (*runnerLockedFile) Close()                       {}
