# goconfig

config manager for Golang applications

## Installation

You can use _go get_:

```bash
go get -u github.com/majiddarvishan/goconfig
```

## Usage
```go
manager, _ := config.NewManager(source)
```

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

    // Query data
    activeUsers, _ := manager.Query("/users/[?active==true]")
    fmt.Printf("Active users: %d\n", len(activeUsers))

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


## Thanks

I am sincerely grateful to `Mohammad Nejati` for his main idea implementation in `C++`
