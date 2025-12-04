# Configuration Manager - Feature Documentation

## Table of Contents
1. [Path Caching](#path-caching)
2. [Change History](#change-history)
3. [Transaction Support](#transaction-support)
4. [Batch Operations](#batch-operations)
5. [Query/Search Functionality](#querysearch-functionality)
6. [Backup/Restore](#backuprestore)
7. [Configuration Validation Service](#configuration-validation-service)
8. [Concurrent Modification Detection](#concurrent-modification-detection)

---

## Path Caching

Automatically maintains a cache of node-to-path mappings for faster lookups in deep configuration trees.

### Features
- **Automatic Management**: Cache is automatically built and invalidated
- **Performance**: O(1) lookups vs O(depth) traversals
- **Thread-Safe**: Protected by manager's mutex

### Usage
```go
// Path caching is automatic - no configuration needed
manager, _ := config.NewManager(source)

// Lookups automatically use cache after first build
// Cache is invalidated after any modification
```

### Performance Impact
- First lookup: O(n) to build cache
- Subsequent lookups: O(1)
- After modification: Cache rebuilt on next lookup

---

## Change History

Maintains a circular buffer of configuration changes with timestamps and details.

### Features
- **Circular Buffer**: Configurable size (default 1000 events)
- **Detailed Events**: Operation type, path, old/new values, version
- **Filtering**: Get history by path or time range
- **Export**: Export history as JSON

### API

```go
// Enable/disable history tracking
manager.EnableHistory(true)

// Get all history
history := manager.GetHistory()
for _, event := range history {
    fmt.Printf("%s: %s at %s (v%d)\n",
        event.Operation, event.Path, event.Timestamp, event.Version)
}

// Get history for specific path
pathHistory := manager.GetHistoryByPath("/users", 10)

// Clear history
manager.ClearHistory()

// Export as JSON
historyJSON, _ := manager.history.ExportJSON()
```

### Event Structure
```go
type ChangeEvent struct {
    Timestamp time.Time   // When change occurred
    Operation string      // "insert", "remove", "replace"
    Path      string      // Path that changed
    Index     *int        // Array index (for insert/remove)
    OldValue  interface{} // Previous value
    NewValue  interface{} // New value
    User      string      // Optional user identifier
    Version   int64       // Version after change
}
```

---

## Transaction Support

Provides atomic multi-operation changes with automatic rollback on failure.

### Features
- **Atomicity**: All operations succeed or all fail
- **Rollback**: Automatic restore on any error
- **Type-Safe**: Operations validated before execution

### API

```go
// Using Batch helper
err := manager.Batch(func(tx *config.Transaction) error {
    tx.Insert("/items", 0, newItem)
    tx.Replace("/count", 5)
    tx.Remove("/temp", 0)
    return nil // All succeed or all rollback
})

// Manual transaction
tx := manager.BeginTransaction()
tx.Insert("/users", 0, user1)
tx.Insert("/users", 1, user2)
if someCondition {
    tx.Rollback() // Discard all operations
} else {
    tx.Commit() // Apply all operations atomically
}

// Batch operations from list
operations := []config.Operation{
    {Type: "insert", Path: "/items", Index: 0, Value: item},
    {Type: "replace", Path: "/status", Value: "active"},
}
manager.BatchOperations(operations)
```

### Error Handling
```go
err := manager.Batch(func(tx *config.Transaction) error {
    tx.Insert("/users", 0, user)
    // If this fails, the insert is rolled back
    tx.Replace("/invalid/path", value)
    return nil
})
if err != nil {
    // Transaction was rolled back
    fmt.Printf("Transaction failed: %v\n", err)
}
```

---

## Batch Operations

Execute multiple operations efficiently under a single lock.

### Features
- **Single Lock**: All operations under one mutex acquisition
- **Atomic**: All succeed or all fail
- **Efficient**: Reduces lock contention

### API

```go
// From operation list
operations := []config.Operation{
    {Type: "insert", Path: "/items", Index: 0, Value: data1},
    {Type: "insert", Path: "/items", Index: 1, Value: data2},
    {Type: "replace", Path: "/count", Value: 2},
}
err := manager.BatchOperations(operations)

// Using transaction builder
err := manager.Batch(func(tx *config.Transaction) error {
    for _, item := range items {
        tx.Insert("/collection", 0, item)
    }
    return nil
})
```

---

## Query/Search Functionality

Powerful query language for searching configuration trees.

### Query Syntax

```
/path/to/key           # Direct path
/users/*/name          # Wildcard - any key
/items/[*]             # All array elements
/items/[0]             # Specific array index
/users/[?age>18]       # Filter by condition
/users/[?active==true] # Boolean filter
```

### Supported Operators
- `==` - Equality
- `!=` - Not equal
- `>` - Greater than
- `<` - Less than
- `>=` - Greater or equal
- `<=` - Less or equal

### API

```go
// Direct path query
results, _ := manager.Query("/settings/timeout")

// Wildcard query
allNames, _ := manager.Query("/users/*/name")

// Array elements
allUsers, _ := manager.Query("/users/[*]")

// Filtered query
activeUsers, _ := manager.Query("/users/[?active==true]")
adults, _ := manager.Query("/users/[?age>=18]")

// Single result
result, _ := manager.QueryOne("/settings/max_users")
value, _ := result.Node.GetInt()

// Custom predicate
stringNodes := manager.FindAll(func(n *config.Node) bool {
    return n.Type() == config.String
})

// Existence check
exists := manager.QueryExists("/optional/feature")

// Count results
count, _ := manager.QueryCount("/items/[*]")
```

### Result Structure
```go
type QueryResult struct {
    Path string      // Full path to node
    Node *Node       // Deep copy of node
}
```

### Examples

```go
// Find all inactive users
inactive, _ := manager.Query("/users/[?active==false]")
for _, result := range inactive {
    obj, _ := result.Node.GetObject()
    name, _ := obj["name"].GetString()
    fmt.Printf("Inactive user: %s at %s\n", name, result.Path)
}

// Find users over 25
oldUsers, _ := manager.Query("/users/[?age>25]")

// Get specific nested value
timeout, _ := manager.QueryOne("/settings/network/timeout")
val, _ := timeout.Node.GetInt()
```

---

## Backup/Restore

Create point-in-time snapshots and restore configuration state.

### Features
- **Full Snapshots**: Captures complete config state
- **Versioned**: Each snapshot includes version number
- **Import/Export**: JSON serialization
- **Auto Backup**: Periodic automatic snapshots

### API

```go
// Manual snapshot
snapshot, err := manager.CreateSnapshot()
fmt.Printf("Created snapshot at v%d\n", snapshot.Version)

// Restore from snapshot
err = manager.Restore(snapshot)

// Export snapshot
snapshotJSON, _ := snapshot.ExportJSON()
saveToFile(snapshotJSON)

// Import snapshot
data := loadFromFile()
imported, _ := config.ImportSnapshot(data)
manager.Restore(imported)

// Auto backup
autoBackup := manager.StartAutoBackup(
    5*time.Minute,  // Interval
    10,             // Keep last 10 snapshots
)
defer autoBackup.Stop()

// Access auto backups
snapshots := autoBackup.GetSnapshots()
latest := autoBackup.GetLatest()
```

### Snapshot Structure
```go
type Snapshot struct {
    Timestamp    time.Time
    Version      int64
    ConfigData   *orderedmap.OrderedMap
    ConfigString string
    Schema       string
}
```

### Use Cases

```go
// Rollback pattern
snapshot, _ := manager.CreateSnapshot()
err := doRiskyOperation(manager)
if err != nil {
    manager.Restore(snapshot)
    fmt.Println("Operation failed, rolled back")
}

// Disaster recovery
autoBackup := manager.StartAutoBackup(1*time.Hour, 24)
// Later, if config is corrupted:
latest := autoBackup.GetLatest()
manager.Restore(latest)
```

---

## Configuration Validation Service

External HTTP-based validation and custom validation rules.

### External Validation

```go
// Setup validation service
validationService := config.NewValidationService(
    "http://validation.example.com/validate",
    10*time.Second, // Timeout
)
validationService.SetHeader("Authorization", "Bearer token")
manager.SetValidationService(validationService)

// Validate current config
ctx := context.Background()
configObj := manager.Source().getConfigObject()
schemaStr := manager.Source().getSchema()
err := validationService.Validate(ctx, configObj, schemaStr)
```

### Custom Validators

```go
// Add custom validation rules
manager.AddValidator("/settings/port",
    config.ValidateRange(1, 65535))

manager.AddValidator("/users/*/email",
    config.ValidatePattern("*@*.*"))

manager.AddValidator("/users/*/role",
    config.ValidateEnum("admin", "user", "guest"))

manager.AddValidator("/database/host",
    config.ValidateRequired())

// Validate array uniqueness
manager.AddValidator("/users",
    config.ValidateUnique("email"))

// Custom validator function
manager.AddValidator("/settings/feature_flags", func(path string, old, new *config.Node) error {
    flags, _ := new.GetObject()
    if len(flags) > 50 {
        return fmt.Errorf("too many feature flags (max 50)")
    }
    return nil
})
```

### Built-in Validators

```go
ValidateRange(min, max float64)        // Numeric range
ValidatePattern(pattern string)        // String pattern
ValidateEnum(values ...interface{})    // Allowed values
ValidateRequired()                     // Non-null/empty
ValidateUnique(field string)           // Array uniqueness
```

### Validation Flow

1. Custom validators run first
2. JSON schema validation
3. External validation service (if configured)
4. If any fail, operation is rolled back

---

## Concurrent Modification Detection

Optimistic locking and conflict detection for concurrent access.

### Features
- **Version-Based**: Each modification increments version
- **Optimistic Locking**: Check version before applying changes
- **Automatic Retry**: Configurable retry strategies
- **Conflict Errors**: Detailed conflict information

### API

```go
// Get current version
version := manager.Version()

// Conditional operations
err := manager.ConditionalReplace("/path", version, newValue)
err := manager.ConditionalInsert("/path", index, version, value)
err := manager.ConditionalRemove("/path", index, version)

// Compare-and-swap
err := manager.CompareAndSwap("/path", expectedVersion, newValue)

// Check for conflict
if config.IsConflictError(err) {
    conflict, _ := config.GetConflictError(err)
    fmt.Printf("Conflict: expected v%d, current v%d\n",
        conflict.YourVersion, conflict.CurrentVersion)
}

// Optimistic update with automatic retry
err := manager.OptimisticUpdate("/counter", func(current *config.Node) (interface{}, error) {
    count, _ := current.GetInt()
    return count + 1, nil // Increment
})

// Manual retry with custom strategy
detector := config.NewConflictDetector(manager)
strategy := config.RetryStrategy{
    MaxAttempts: 5,
    OnConflict: func(attempt int, err error) bool {
        time.Sleep(time.Millisecond * 100) // Backoff
        return attempt < 5
    },
}

err = detector.RetryOnConflict(strategy, func(expectedVersion int64) error {
    return manager.ConditionalReplace("/path", expectedVersion, value)
})
```

### Conflict Error Structure

```go
type ConflictError struct {
    Path           string
    Operation      string
    YourVersion    int64
    CurrentVersion int64
    CurrentValue   interface{}
    Message        string
}
```

### Concurrent Access Pattern

```go
// Goroutine 1
go func() {
    version := manager.Version()
    // ... do some computation ...
    err := manager.ConditionalReplace("/shared", version, result1)
    if config.IsConflictError(err) {
        // Retry logic
    }
}()

// Goroutine 2
go func() {
    version := manager.Version()
    // ... do some computation ...
    err := manager.ConditionalReplace("/shared", version, result2)
    if config.IsConflictError(err) {
        // Retry logic
    }
}()
```

### Optimistic Locking Pattern

```go
func updateWithRetry(manager *config.Manager, path string, updateFn func(int) int) error {
    for attempt := 0; attempt < 3; attempt++ {
        // Read current value and version
        version := manager.Version()
        results, _ := manager.Query(path)
        if len(results) == 0 {
            return fmt.Errorf("path not found")
        }

        currentVal, _ := results[0].Node.GetInt()
        newVal := updateFn(currentVal)

        // Try conditional update
        err := manager.ConditionalReplace(path, version, newVal)
        if err == nil {
            return nil // Success
        }

        if !config.IsConflictError(err) {
            return err // Non-conflict error
        }

        // Conflict - retry
        time.Sleep(time.Millisecond * 50)
    }
    return fmt.Errorf("failed after 3 attempts")
}
```

---

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "time"
    "your-module/config"
)

func main() {
    // Initialize
    source, _ := config.NewStrSource(configJSON, schemaJSON)
    manager, _ := config.NewManager(source)

    // Setup features
    manager.EnableHistory(true)
    manager.AddValidator("/settings/port", config.ValidateRange(1, 65535))

    validationSvc := config.NewValidationService("http://validator:8080", 5*time.Second)
    manager.SetValidationService(validationSvc)

    autoBackup := manager.StartAutoBackup(10*time.Minute, 10)
    defer autoBackup.Stop()

    // Atomic transaction
    err := manager.Batch(func(tx *config.Transaction) error {
        tx.Insert("/users", 0, newUser)
        tx.Replace("/settings/max_users", 150)
        return nil
    })

    // Query data
    activeUsers, _ := manager.Query("/users/[?active==true]")
    fmt.Printf("Active users: %d\n", len(activeUsers))

    // Optimistic update
    err = manager.OptimisticUpdate("/counter", func(current *config.Node) (interface{}, error) {
        count, _ := current.GetInt()
        return count + 1, nil
    })

    // View history
    history := manager.GetHistory()
    for _, event := range history {
        fmt.Printf("%s: %s\n", event.Operation, event.Path)
    }

    // Backup/restore
    snapshot, _ := manager.CreateSnapshot()
    // ... later ...
    manager.Restore(snapshot)
}
```

---

## Performance Considerations

### Path Caching
- **Best for**: Deep trees with frequent lookups
- **Memory**: O(n) where n is number of nodes
- **Rebuild cost**: O(n) after modifications

### Change History
- **Memory**: Fixed size circular buffer
- **Write cost**: O(1) per operation
- **Query cost**: O(h) where h is history size

### Transactions
- **Lock duration**: Entire transaction duration
- **Rollback**: Fast (in-memory only)
- **Best for**: Multiple related changes

### Queries
- **Simple path**: O(depth)
- **Wildcard**: O(n) where n is matching nodes
- **Filter**: O(n) where n is array size
- **Cache**: Results not cached, evaluate each time

### Snapshots
- **Creation**: O(n) - full tree clone
- **Restore**: O(n) - full tree replacement
- **Storage**: Full config copy per snapshot

---

## Thread Safety

All features are thread-safe:
- **Manager**: Protected by RWMutex
- **History**: Protected by manager's mutex
- **Transactions**: Exclusive lock during execution
- **Queries**: Read lock during execution
- **Snapshots**: Independent copies, safe to use

---

## Error Handling

### Error Types
```go
// Conflict errors
if config.IsConflictError(err) {
    conflict, _ := config.GetConflictError(err)
    // Handle conflict
}

// Validation errors
if err != nil {
    fmt.Printf("Validation failed: %v\n", err)
}

// Transaction rollback
err := manager.Batch(func(tx *config.Transaction) error {
    // Any error here causes rollback
    return someError
})
```

### Best Practices
1. Always check errors
2. Use transactions for related changes
3. Handle conflicts with retry logic
4. Create snapshots before risky operations
5. Use custom validators for business rules
6. Enable history for debugging
7. Use queries instead of manual traversal
8. Configure auto-backup for production

---

## Migration from Previous Version

```go
// Old code
clone := Clone(om)  // Could be nil
jsonSetByPath(om, path, value)  // Returns bool

// New code
clone, err := Clone(om)  // Check error
if err != nil { /* handle */ }
err = jsonSetByPath(om, path, value)  // Returns error
if err != nil { /* handle */ }

// Old HTTP API
{"op": "replace", "path": "/key", "config_hash": "abc123"}

// New HTTP API
{"op": "replace", "path": "/key", "version": 42}
```