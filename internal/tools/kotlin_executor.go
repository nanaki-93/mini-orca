package tools

import (
	"context"
	"fmt"
)

// kotlinExecutor implements Kotlin-specific operations.
type kotlinExecutor struct {
	shell Executor
}

// NewKotlinExecutor creates a new Kotlin executor instance.
func NewKotlinExecutor() *kotlinExecutor {
	return &kotlinExecutor{
		shell: NewShellExecutor(),
	}
}

// FormatCode runs `./gradlew ktlintFormat` to format Kotlin code.
func (k *kotlinExecutor) FormatCode() error {
	_, err := k.shell.Shell(context.Background(), "./gradlew", "ktlintFormat")
	if err != nil {
		return fmt.Errorf("kotlin_executor: gradlew ktlintFormat failed: %w", err)
	}
	return nil
}

// RunTests runs `./gradlew test` to execute Kotlin tests.
func (k *kotlinExecutor) RunTests() (*ShellResult, error) {
	result, err := k.shell.Shell(context.Background(), "./gradlew", "test")
	if err != nil {
		return result, fmt.Errorf("kotlin_executor: gradlew test failed: %w", err)
	}
	return result, nil
}

// Build runs `./gradlew build` to build the Kotlin project.
func (k *kotlinExecutor) Build() error {
	_, err := k.shell.Shell(context.Background(), "./gradlew", "build")
	if err != nil {
		return fmt.Errorf("kotlin_executor: gradlew build failed: %w", err)
	}
	return nil
}

// Lint runs `./gradlew ktlintCheck` to lint Kotlin code.
func (k *kotlinExecutor) Lint() (*ShellResult, error) {
	return k.shell.Shell(context.Background(), "./gradlew", "ktlintCheck")
}
