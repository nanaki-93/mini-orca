package tools

import (
	"context"
	"fmt"
)

// javaExecutor implements Java-specific operations.
type javaExecutor struct {
	shell Executor
}

// NewJavaExecutor creates a new Java executor instance.
func NewJavaExecutor() *javaExecutor {
	return &javaExecutor{
		shell: NewShellExecutor(),
	}
}

// FormatCode runs `./gradlew spotlessApply` to format Java code.
func (j *javaExecutor) FormatCode() error {
	_, err := j.shell.Shell(context.Background(), "./gradlew", "spotlessApply")
	if err != nil {
		return fmt.Errorf("java_executor: gradlew spotlessApply failed: %w", err)
	}
	return nil
}

// RunTests runs `./gradlew test` to execute Java tests.
func (j *javaExecutor) RunTests() (*ShellResult, error) {
	result, err := j.shell.Shell(context.Background(), "./gradlew", "test")
	if err != nil {
		return result, fmt.Errorf("java_executor: gradlew test failed: %w", err)
	}
	return result, nil
}

// Build runs `./gradlew build` to build the Java project.
func (j *javaExecutor) Build() error {
	_, err := j.shell.Shell(context.Background(), "./gradlew", "build")
	if err != nil {
		return fmt.Errorf("java_executor: gradlew build failed: %w", err)
	}
	return nil
}

// Lint runs `./gradlew spotlessCheck` to lint Java code.
func (j *javaExecutor) Lint() (*ShellResult, error) {
	return j.shell.Shell(context.Background(), "./gradlew", "spotlessCheck")
}
