package internal

import (
	"fmt"
)

// ConflictError represents a concurrent modification conflict
type ConflictError struct {
	Path           string      `json:"path"`
	Operation      string      `json:"operation"`
	YourVersion    int64       `json:"your_version"`
	CurrentVersion int64       `json:"current_version"`
	CurrentValue   interface{} `json:"current_value,omitempty"`
	Message        string      `json:"message"`
}

func (e *ConflictError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf(
		"conflict at %s: version mismatch (expected %d, current %d)",
		e.Path, e.YourVersion, e.CurrentVersion,
	)
}

// NewConflictError creates a new conflict error
func NewConflictError(path, operation string, yourVersion, currentVersion int64) *ConflictError {
	return &ConflictError{
		Path:           path,
		Operation:      operation,
		YourVersion:    yourVersion,
		CurrentVersion: currentVersion,
		Message: fmt.Sprintf(
			"conflict: %s operation at %s failed - version %d expected but current is %d",
			operation, path, yourVersion, currentVersion,
		),
	}
}

// ConflictDetector provides methods to detect and handle conflicts
type ConflictDetector struct {
	manager *Manager
}

// NewConflictDetector creates a new conflict detector
func NewConflictDetector(m *Manager) *ConflictDetector {
	return &ConflictDetector{manager: m}
}

// CheckVersion verifies that the expected version matches current version
func (cd *ConflictDetector) CheckVersion(expectedVersion int64) error {
	currentVersion := cd.manager.Version()
	if currentVersion != expectedVersion {
		return NewConflictError("", "check", expectedVersion, currentVersion)
	}
	return nil
}

// TryOperation attempts an operation with conflict detection
func (cd *ConflictDetector) TryOperation(
	expectedVersion int64,
	op func() error,
) error {
	if err := cd.CheckVersion(expectedVersion); err != nil {
		return err
	}
	return op()
}

// RetryStrategy defines how to retry conflicting operations
type RetryStrategy struct {
	MaxAttempts int
	OnConflict  func(attempt int, err error) bool // Return true to retry
}

// DefaultRetryStrategy provides a sensible default retry strategy
func DefaultRetryStrategy() RetryStrategy {
	return RetryStrategy{
		MaxAttempts: 3,
		OnConflict: func(attempt int, err error) bool {
			return attempt < 3 // Retry up to 3 times
		},
	}
}

// RetryOnConflict retries an operation if it encounters a conflict
func (cd *ConflictDetector) RetryOnConflict(
	strategy RetryStrategy,
	op func(currentVersion int64) error,
) error {
	var lastErr error

	for attempt := 0; attempt < strategy.MaxAttempts; attempt++ {
		currentVersion := cd.manager.Version()
		err := op(currentVersion)

		if err == nil {
			return nil
		}

		// Check if it's a conflict error
		if _, isConflict := err.(*ConflictError); !isConflict {
			return err // Not a conflict, fail immediately
		}

		lastErr = err

		// Check if we should retry
		if !strategy.OnConflict(attempt+1, err) {
			break
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", strategy.MaxAttempts, lastErr)
}

// CompareAndSwap performs a compare-and-swap operation
func (m *Manager) CompareAndSwap(path string, expectedVersion int64, newValue interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.version != expectedVersion {
		// Get current value for better error message
		var currentValue interface{}
		results, err := m.queryLocked(path)
		if err == nil && len(results) > 0 {
			currentValue = results[0].Node.value
		}

		return &ConflictError{
			Path:           path,
			Operation:      "compare-and-swap",
			YourVersion:    expectedVersion,
			CurrentVersion: m.version,
			CurrentValue:   currentValue,
		}
	}

	return m.replaceLocked(path, newValue)
}

// ConditionalInsert inserts only if the version hasn't changed
func (m *Manager) ConditionalInsert(path string, index int, expectedVersion int64, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.version != expectedVersion {
		return NewConflictError(path, "insert", expectedVersion, m.version)
	}

	return m.insertLocked(path, index, value)
}

// ConditionalRemove removes only if the version hasn't changed
func (m *Manager) ConditionalRemove(path string, index int, expectedVersion int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.version != expectedVersion {
		return NewConflictError(path, "remove", expectedVersion, m.version)
	}

	return m.removeLocked(path, index)
}

// ConditionalReplace replaces only if the version hasn't changed
func (m *Manager) ConditionalReplace(path string, expectedVersion int64, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.version != expectedVersion {
		return NewConflictError(path, "replace", expectedVersion, m.version)
	}

	return m.replaceLocked(path, value)
}

// OptimisticUpdate provides a higher-level optimistic update pattern
func (m *Manager) OptimisticUpdate(
	path string,
	updateFn func(current *Node) (interface{}, error),
) error {
	detector := NewConflictDetector(m)
	strategy := DefaultRetryStrategy()

	return detector.RetryOnConflict(strategy, func(expectedVersion int64) error {
		// Read current value
		m.mu.RLock()
		results, err := m.queryLocked(path)
		m.mu.RUnlock()

		if err != nil || len(results) == 0 {
			return fmt.Errorf("path not found: %s", path)
		}

		current := results[0].Node

		// Compute new value
		newValue, err := updateFn(current)
		if err != nil {
			return err
		}

		// Try to apply with version check
		return m.ConditionalReplace(path, expectedVersion, newValue)
	})
}

// IsConflictError checks if an error is a conflict error
func IsConflictError(err error) bool {
	_, ok := err.(*ConflictError)
	return ok
}

// GetConflictError extracts conflict error details if possible
func GetConflictError(err error) (*ConflictError, bool) {
	ce, ok := err.(*ConflictError)
	return ce, ok
}