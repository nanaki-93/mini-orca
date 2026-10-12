package app

// scopedRuntimes owns the immutable runtime selected for each fixed workflow
// scope. Runtime profiles are established during service construction.
type scopedRuntimes struct {
	analyze  modelRuntime
	bug      modelRuntime
	function modelRuntime
}

func (r scopedRuntimes) forScope(scope string) (modelRuntime, bool) {
	switch scope {
	case r.analyze.effective.Scope:
		return r.analyze, true
	case r.bug.effective.Scope:
		return r.bug, true
	case r.function.effective.Scope:
		return r.function, true
	default:
		return modelRuntime{}, false
	}
}
