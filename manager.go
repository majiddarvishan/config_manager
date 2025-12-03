package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

type handler_t func(*Node)

type modifiableType int

const (
	Insertable modifiableType = iota
	Removable
	Replaceable
)

type modifiable struct {
	Type    modifiableType
	Path    string
	Node    *Node
	Handler handler_t
}

type Manager struct {
	mu sync.RWMutex

	source      ISource
	config      *Node
	modifiables []modifiable
	version     int64 // Version counter for optimistic locking
}

func NewManager(source ISource) (*Manager, error) {
	if source == nil {
		return nil, errors.New("source cannot be nil")
	}

	root := parseNode(source.getConfigObject())
	if root == nil {
		return nil, errors.New("failed to parse config root")
	}

	m := &Manager{
		source:      source,
		config:      root,
		modifiables: make([]modifiable, 0),
		version:     1,
	}

	if err := validate(source.getConfig(), source.getSchema()); err != nil {
		return nil, fmt.Errorf("initial config validation failed: %w", err)
	}

	return m, nil
}

// Config returns a deep copy of the config to prevent data races
func (m *Manager) Config() *Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// return m.config.DeepCopy()
    return m.config
}

func (m *Manager) Source() ISource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.source
}

func (m *Manager) Version() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.version
}

////////////////////////////////////////////////////////////////////////////////
// INSERT (improved with proper rollback)
////////////////////////////////////////////////////////////////////////////////

func (m *Manager) insert(path string, index int, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mod, err := m.findModifiableLocked(Insertable, path)
	if err != nil {
		return err
	}

	// Validate index bounds first
	array, err := mod.Node.GetArray()
	if err != nil {
		return err
	}
	if index < 0 || index > len(array) {
		return fmt.Errorf("index %d out of bounds [0,%d]", index, len(array))
	}

	// Clone and validate
	jsonConfig, err := Clone(m.source.getConfigObject())
	if err != nil {
		return fmt.Errorf("failed to clone config: %w", err)
	}

	if err := jsonInsertByPath(jsonConfig, path, index, value); err != nil {
		return fmt.Errorf("failed to insert: %w", err)
	}

	if err := validateJSONAgainstSchema(jsonConfig, m.source.getSchema()); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Create backup for rollback
	oldArray := make([]*Node, len(array))
	copy(oldArray, array)

	// Mutate in-memory node
	newNode := parseNode(value)
	newArr := make([]*Node, 0, len(array)+1)
	newArr = append(newArr, array[:index]...)
	newArr = append(newArr, newNode)
	newArr = append(newArr, array[index:]...)
	*mod.Node = Node{newArr}

	// Persist changes
	if err := m.source.setConfig(jsonConfig); err != nil {
		// Rollback on failure
		*mod.Node = Node{oldArray}
		return fmt.Errorf("failed to persist config: %w", err)
	}

	m.version++
	m.updateModifiablesLocked()

	// Call handler AFTER successful persistence, outside of critical section
	handler := mod.Handler
	handlerNode := newNode

	if handler != nil {
		// Unlock during handler execution to avoid deadlocks
		// Handler gets a copy so it can't corrupt state
		m.mu.Unlock()
		// handler(handlerNode.DeepCopy())
        handler(handlerNode)
		m.mu.Lock()
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// REMOVE (improved with proper rollback)
////////////////////////////////////////////////////////////////////////////////

func (m *Manager) remove(path string, index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mod, err := m.findModifiableLocked(Removable, path)
	if err != nil {
		return err
	}

	array, err := mod.Node.GetArray()
	if err != nil {
		return err
	}

	if index < 0 || index >= len(array) {
		return fmt.Errorf("index %d out of bounds [0,%d)", index, len(array))
	}

	jsonConfig, err := Clone(m.source.getConfigObject())
	if err != nil {
		return fmt.Errorf("failed to clone config: %w", err)
	}

	if err := jsonRemoveByPath(jsonConfig, path, index); err != nil {
		return fmt.Errorf("failed to remove: %w", err)
	}

	if err := validateJSONAgainstSchema(jsonConfig, m.source.getSchema()); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Backup for rollback
	oldArray := make([]*Node, len(array))
	copy(oldArray, array)
	removedNode := array[index]

	// Mutate
	newArr := make([]*Node, 0, len(array)-1)
	newArr = append(newArr, array[:index]...)
	newArr = append(newArr, array[index+1:]...)
	*mod.Node = Node{newArr}

	// Persist
	if err := m.source.setConfig(jsonConfig); err != nil {
		*mod.Node = Node{oldArray}
		return fmt.Errorf("failed to persist config: %w", err)
	}

	m.version++
	m.updateModifiablesLocked()

	handler := mod.Handler
	handlerNode := removedNode

	if handler != nil {
		m.mu.Unlock()
		// handler(handlerNode.DeepCopy())
        handler(handlerNode)
		m.mu.Lock()
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// REPLACE (improved with proper rollback)
////////////////////////////////////////////////////////////////////////////////

func (m *Manager) replace(path string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mod, err := m.findModifiableLocked(Replaceable, path)
	if err != nil {
		return err
	}

	jsonConfig, err := Clone(m.source.getConfigObject())
	if err != nil {
		return fmt.Errorf("failed to clone config: %w", err)
	}

	if err := jsonSetByPath(jsonConfig, path, value); err != nil {
		return fmt.Errorf("failed to set: %w", err)
	}

	if err := validateJSONAgainstSchema(jsonConfig, m.source.getSchema()); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Backup for rollback
	oldNode := *mod.Node

	// Mutate
	newNode := parseNode(value)
	*mod.Node = *newNode

	// Persist
	if err := m.source.setConfig(jsonConfig); err != nil {
		*mod.Node = oldNode
		return fmt.Errorf("failed to persist config: %w", err)
	}

	m.version++
	m.updateModifiablesLocked()

	handler := mod.Handler
	handlerNode := mod.Node

	if handler != nil {
		m.mu.Unlock()
		// handler(handlerNode.DeepCopy())
        handler(handlerNode)
		m.mu.Lock()
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// REGISTRATION
////////////////////////////////////////////////////////////////////////////////

func (m *Manager) OnInsert(node *Node, handler handler_t) error {
	if node == nil {
		return errors.New("node cannot be nil")
	}
	if node.Type() != Array {
		return errors.New("node must be array for insert operations")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	p, err := m.findAndSanitizeNodePathLocked(node)
	if err != nil {
		return err
	}

	m.modifiables = append(m.modifiables, modifiable{
		Type:    Insertable,
		Path:    p,
		Node:    node,
		Handler: handler,
	})

	return nil
}

func (m *Manager) OnRemove(node *Node, handler handler_t) error {
	if node == nil {
		return errors.New("node cannot be nil")
	}
	if node.Type() != Array {
		return errors.New("node must be array for remove operations")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	p, err := m.findAndSanitizeNodePathLocked(node)
	if err != nil {
		return err
	}

	m.modifiables = append(m.modifiables, modifiable{
		Type:    Removable,
		Path:    p,
		Node:    node,
		Handler: handler,
	})

	return nil
}

func (m *Manager) OnReplace(node *Node, handler handler_t) error {
	if node == nil {
		return errors.New("node cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	p, err := m.findAndSanitizeNodePathLocked(node)
	if err != nil {
		return err
	}

	m.modifiables = append(m.modifiables, modifiable{
		Type:    Replaceable,
		Path:    p,
		Node:    node,
		Handler: handler,
	})

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PATH HELPERS
////////////////////////////////////////////////////////////////////////////////

func (m *Manager) getInsertablePaths() []string  { return m.getPathsLocked(Insertable) }
func (m *Manager) getRemovablePaths() []string   { return m.getPathsLocked(Removable) }
func (m *Manager) getReplaceablePaths() []string { return m.getPathsLocked(Replaceable) }

func (m *Manager) getPathsLocked(t modifiableType) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]string, 0, len(m.modifiables))
	for _, v := range m.modifiables {
		if v.Type == t {
			out = append(out, v.Path)
		}
	}
	return out
}

func (m *Manager) findModifiableLocked(t modifiableType, path string) (*modifiable, error) {
	for i := range m.modifiables {
		if m.modifiables[i].Type == t && m.modifiables[i].Path == path {
			return &m.modifiables[i], nil
		}
	}
	return nil, fmt.Errorf("path '%s' not modifiable for operation type %d", path, t)
}

func (m *Manager) updateModifiablesLocked() {
	// Remove invalid modifiables
	validMods := make([]modifiable, 0, len(m.modifiables))
	for _, mod := range m.modifiables {
		if path := m.findNodePathLocked(mod.Node); path != "" {
			mod.Path = path
			validMods = append(validMods, mod)
		}
	}
	m.modifiables = validMods
}

func (m *Manager) findNodePathLocked(n *Node) string {
	return findNodePath(m.config, n)
}

func (m *Manager) findAndSanitizeNodePathLocked(n *Node) (string, error) {
	p := m.findNodePathLocked(n)
	if p == "" {
		return "", errors.New("node has no valid path in config tree")
	}
	return p, nil
}

////////////////////////////////////////////////////////////////////////////////
// HELPERS
////////////////////////////////////////////////////////////////////////////////

func validateJSONAgainstSchema(obj interface{}, schema *string) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}
	s := string(b)
	return validate(&s, schema)
}