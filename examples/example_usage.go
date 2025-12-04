package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"your-module/config"
)

func main() {
	// Example configuration and schema
	configJSON := `{
		"users": [
			{"name": "Alice", "age": 30, "active": true},
			{"name": "Bob", "age": 25, "active": false}
		],
		"settings": {
			"max_users": 100,
			"timeout": 30
		}
	}`

	schemaJSON := `{
		"type": "object",
		"properties": {
			"users": {"type": "array"},
			"settings": {"type": "object"}
		}
	}`

	// Create manager
	source, _ := config.NewStrSource(configJSON, schemaJSON)
	manager, err := config.NewManager(source)
	if err != nil {
		log.Fatal(err)
	}

	// ========== PATH CACHING ==========
	fmt.Println("\n=== Path Caching Demo ===")
	// Path cache is automatically maintained
	// Significantly faster lookups for deep trees

	// ========== CHANGE HISTORY ==========
	fmt.Println("\n=== Change History Demo ===")

	// History is enabled by default
	manager.EnableHistory(true)

	// Make some changes
	manager.Batch(func(tx *config.Transaction) error {
		tx.Replace("/settings/timeout", 60)
		return nil
	})

	// View history
	history := manager.GetHistory()
	fmt.Printf("Total changes: %d\n", len(history))
	for _, event := range history {
		fmt.Printf("  %s: %s at %s\n", event.Operation, event.Path, event.Timestamp)
	}

	// View history for specific path
	pathHistory := manager.GetHistoryByPath("/settings/timeout", 10)
	fmt.Printf("Changes to /settings/timeout: %d\n", len(pathHistory))

	// ========== TRANSACTION SUPPORT ==========
	fmt.Println("\n=== Transaction Demo ===")

	// Atomic multi-operation
	err = manager.Batch(func(tx *config.Transaction) error {
		tx.Insert("/users", 0, map[string]interface{}{
			"name": "Charlie", "age": 35, "active": true,
		})
		tx.Replace("/settings/max_users", 150)
		return nil
	})
	if err != nil {
		fmt.Printf("Batch failed: %v\n", err)
	} else {
		fmt.Println("Batch succeeded - all operations committed atomically")
	}

	// If any operation fails, all are rolled back
	err = manager.Batch(func(tx *config.Transaction) error {
		tx.Insert("/users", 0, map[string]interface{}{"name": "Dave"})
		return fmt.Errorf("simulated error") // This rolls back the insert
	})
	fmt.Printf("Expected error: %v\n", err)

	// ========== BATCH OPERATIONS ==========
	fmt.Println("\n=== Batch Operations Demo ===")

	operations := []config.Operation{
		{Type: "insert", Path: "/users", Index: 1, Value: map[string]interface{}{
			"name": "Eve", "age": 28, "active": true,
		}},
		{Type: "replace", Path: "/settings/timeout", Value: 45},
	}

	err = manager.BatchOperations(operations)
	if err != nil {
		fmt.Printf("Batch operations failed: %v\n", err)
	} else {
		fmt.Println("Batch operations succeeded")
	}

	// ========== QUERY/SEARCH ==========
	fmt.Println("\n=== Query Demo ===")

	// Direct path query
	results, _ := manager.Query("/users/0/name")
	if len(results) > 0 {
		fmt.Printf("First user name: %v\n", results[0].Node)
	}

	// Wildcard query - all user names
	results, _ = manager.Query("/users/*/name")
	fmt.Printf("All user names (%d results):\n", len(results))
	for _, r := range results {
		name, _ := r.Node.GetString()
		fmt.Printf("  - %s\n", name)
	}

	// Array wildcard - all users
	results, _ = manager.Query("/users/[*]")
	fmt.Printf("All users: %d\n", len(results))

	// Filter query - active users
	results, _ = manager.Query("/users/[?active==true]")
	fmt.Printf("Active users: %d\n", len(results))
	for _, r := range results {
		obj, _ := r.Node.GetObject()
		name, _ := obj["name"].GetString()
		fmt.Printf("  - %s (active)\n", name)
	}

	// Numeric filter - users over 27
	results, _ = manager.Query("/users/[?age>27]")
	fmt.Printf("Users over 27: %d\n", len(results))

	// FindAll with custom predicate
	stringNodes := manager.FindAll(func(n *config.Node) bool {
		return n.Type() == config.String
	})
	fmt.Printf("All string nodes: %d\n", len(stringNodes))

	// Query existence check
	exists := manager.QueryExists("/settings/timeout")
	fmt.Printf("Settings timeout exists: %v\n", exists)

	// ========== BACKUP/RESTORE ==========
	fmt.Println("\n=== Backup/Restore Demo ===")

	// Create manual snapshot
	snapshot, err := manager.CreateSnapshot()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created snapshot at version %d\n", snapshot.Version)

	// Make some changes
	manager.Batch(func(tx *config.Transaction) error {
		tx.Replace("/settings/max_users", 200)
		return nil
	})
	fmt.Printf("Changed max_users, current version: %d\n", manager.Version())

	// Restore from snapshot
	err = manager.Restore(snapshot)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Restored snapshot, current version: %d\n", manager.Version())

	// Auto backup
	autoBackup := manager.StartAutoBackup(5*time.Second, 5)
	defer autoBackup.Stop()
	fmt.Println("Started auto backup every 5 seconds, keeping last 5 snapshots")

	// Later: Get snapshots
	time.Sleep(2 * time.Second)
	snapshots := autoBackup.GetSnapshots()
	fmt.Printf("Auto backup has %d snapshots\n", len(snapshots))

	// Export/Import snapshot
	snapshotJSON, _ := snapshot.ExportJSON()
	fmt.Printf("Exported snapshot: %d bytes\n", len(snapshotJSON))

	imported, _ := config.ImportSnapshot(snapshotJSON)
	fmt.Printf("Imported snapshot from version %d\n", imported.Version)

	// ========== CUSTOM VALIDATION ==========
	fmt.Println("\n=== Custom Validation Demo ===")

	// Add custom validators
	manager.AddValidator("/settings/max_users", config.ValidateRange(1, 1000))
	manager.AddValidator("/users/*/age", config.ValidateRange(0, 150))
	manager.AddValidator("/users/*/name", config.ValidateRequired())

	// This will pass validation
	err = manager.Batch(func(tx *config.Transaction) error {
		return tx.Replace("/settings/max_users", 500)
	})
	fmt.Printf("Valid update: %v\n", err)

	// This will fail validation (out of range)
	err = manager.Batch(func(tx *config.Transaction) error {
		return tx.Replace("/settings/max_users", 2000)
	})
	fmt.Printf("Invalid update (expected error): %v\n", err)

	// ========== EXTERNAL VALIDATION SERVICE ==========
	fmt.Println("\n=== External Validation Service Demo ===")

	// Setup external validation service (if available)
	validationService := config.NewValidationService("http://localhost:9000/validate", 5*time.Second)
	validationService.SetHeader("Authorization", "Bearer token123")
	manager.SetValidationService(validationService)

	// Validate current config against external service
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	configObj := manager.Source().getConfigObject()
	schemaStr := manager.Source().getSchema()
	err = validationService.Validate(ctx, configObj, schemaStr)
	if err != nil {
		fmt.Printf("External validation: %v\n", err)
	} else {
		fmt.Println("External validation passed")
	}

	// ========== CONCURRENT MODIFICATION DETECTION ==========
	fmt.Println("\n=== Conflict Detection Demo ===")

	// Get current version
	currentVersion := manager.Version()
	fmt.Printf("Current version: %d\n", currentVersion)

	// Conditional update (optimistic locking)
	err = manager.ConditionalReplace("/settings/timeout", currentVersion, 90)
	if err != nil {
		fmt.Printf("Conditional update failed: %v\n", err)
	} else {
		fmt.Println("Conditional update succeeded")
	}

	// Try with wrong version (will fail)
	err = manager.ConditionalReplace("/settings/timeout", currentVersion, 100)
	if config.IsConflictError(err) {
		conflictErr, _ := config.GetConflictError(err)
		fmt.Printf("Conflict detected: expected v%d, current v%d\n",
			conflictErr.YourVersion, conflictErr.CurrentVersion)
	}

	// Compare-and-swap
	currentVersion = manager.Version()
	err = manager.CompareAndSwap("/settings/timeout", currentVersion, 120)
	if err != nil {
		fmt.Printf("CAS failed: %v\n", err)
	} else {
		fmt.Println("CAS succeeded")
	}

	// Optimistic update with retry
	err = manager.OptimisticUpdate("/settings/max_users", func(current *config.Node) (interface{}, error) {
		currentVal, _ := current.GetInt()
		return currentVal + 10, nil // Increment by 10
	})
	if err != nil {
		fmt.Printf("Optimistic update failed: %v\n", err)
	} else {
		fmt.Println("Optimistic update succeeded (with automatic retry on conflict)")
	}

	// Manual retry with custom strategy
	detector := config.NewConflictDetector(manager)
	strategy := config.RetryStrategy{
		MaxAttempts: 5,
		OnConflict: func(attempt int, err error) bool {
			fmt.Printf("  Retry attempt %d due to conflict\n", attempt)
			return attempt < 5
		},
	}

	err = detector.RetryOnConflict(strategy, func(expectedVersion int64) error {
		return manager.ConditionalReplace("/settings/timeout", expectedVersion, 150)
	})
	if err != nil {
		fmt.Printf("Operation failed after retries: %v\n", err)
	} else {
		fmt.Println("Operation succeeded after retries")
	}

	// ========== COMBINED EXAMPLE ==========
	fmt.Println("\n=== Combined Features Demo ===")

	// Complex workflow using multiple features
	fmt.Println("Starting complex workflow...")

	// 1. Create backup before changes
	preWorkflowSnapshot, _ := manager.CreateSnapshot()

	// 2. Use transaction for atomic operations
	err = manager.Batch(func(tx *config.Transaction) error {
		// Insert new user with validation
		newUser := map[string]interface{}{
			"name": "Frank", "age": 32, "active": true,
		}
		if err := tx.Insert("/users", 0, newUser); err != nil {
			return err
		}

		// Update settings
		if err := tx.Replace("/settings/max_users", 110); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Workflow failed: %v\n", err)
		// Restore from backup
		manager.Restore(preWorkflowSnapshot)
		fmt.Println("Rolled back to pre-workflow state")
	} else {
		fmt.Println("Workflow completed successfully")

		// 3. Query results
		activeUsers, _ := manager.Query("/users/[?active==true]")
		fmt.Printf("Active users after workflow: %d\n", len(activeUsers))

		// 4. View what changed
		recentHistory := manager.GetHistory()
		fmt.Printf("Recent changes: %d\n", len(recentHistory))
	}

	fmt.Println("\n=== Demo Complete ===")
}