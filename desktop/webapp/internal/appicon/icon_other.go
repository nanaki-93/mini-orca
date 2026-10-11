//go:build !darwin

package appicon

// Other hosts use their platform's packaged application icon.
func Set([]byte) error { return nil }
