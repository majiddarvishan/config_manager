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
	Replacable
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
	}

	if err := validate(source.getConfig(), source.getSchema()); err != nil {
		return nil, fmt.Errorf("initial config validation failed: %w", err)
	}

	return m, nil
}

func (m *Manager) Config() *Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

func (m *Manager) Source() ISource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.source
}

////////////////////////////////////////////////////////////////////////////////
// INSERT (two-phase locking)
////////////////////////////////////////////////////////////////////////////////

func (m *Manager) insert(path string, index int, value interface{}) error {
	m.mu.Lock()

	mod, err := m.findModifiable(Insertable, path)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	jsonConfig := Clone(m.source.getConfigObject())
	if !jsonInsertByPath(jsonConfig, path, index, value) {
		m.mu.Unlock()
		return errors.New("could not insert")
	}

	// Validate JSON
	if err := validateJSONAgainstSchema(jsonConfig, m.source.getSchema()); err != nil {
		m.mu.Unlock()
		return err
	}

	// Mutate in-memory node
	newNode := parseNode(value)

	array, err := mod.Node.GetArray()
	if err != nil {
		m.mu.Unlock()
		return err
	}

	if index < 0 || index > len(array) {
		m.mu.Unlock()
		return errors.New("index out of bounds")
	}

	// Copy array
	newArr := append(append(array[:index], newNode), array[index:]...)
	*mod.Node = Node{newArr}

	handler := mod.Handler
	handlerNode := newNode

	// Unlock before handler
	m.mu.Unlock()

	if handler != nil {
		handler(handlerNode)
	}

	// Phase 2 — Persist + update paths
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.source.setConfig(jsonConfig); err != nil {
		return err
	}

	m.updateModifiables()

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// REMOVE (two-phase locking)
////////////////////////////////////////////////////////////////////////////////

func (m *Manager) remove(path string, index int) error {
	m.mu.Lock()

	mod, err := m.findModifiable(Removable, path)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	jsonConfig := Clone(m.source.getConfigObject())
	if !jsonRemoveByPath(jsonConfig, path, index) {
		m.mu.Unlock()
		return errors.New("could not remove")
	}

	if err := validateJSONAgainstSchema(jsonConfig, m.source.getSchema()); err != nil {
		m.mu.Unlock()
		return err
	}

	array, err := mod.Node.GetArray()
	if err != nil {
		m.mu.Unlock()
		return err
	}

	if index < 0 || index >= len(array) {
		m.mu.Unlock()
		return errors.New("index out of bounds")
	}

	removedNode := array[index]

	newArr := append(array[:index], array[index+1:]...)
	*mod.Node = Node{newArr}

	handler := mod.Handler
	handlerNode := removedNode

	// Unlock before handler
	m.mu.Unlock()

	if handler != nil {
		handler(handlerNode)
	}

	// Phase 2 — Persist + update
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.source.setConfig(jsonConfig); err != nil {
		return err
	}

	m.updateModifiables()
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// REPLACE (two-phase locking)
////////////////////////////////////////////////////////////////////////////////

func (m *Manager) replace(path string, value interface{}) error {
	m.mu.Lock()

	mod, err := m.findModifiable(Replacable, path)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	jsonConfig := Clone(m.source.getConfigObject())
	if !jsonSetByPath(jsonConfig, path, value) {
		m.mu.Unlock()
		return errors.New("could not set")
	}

	if err := validateJSONAgainstSchema(jsonConfig, m.source.getSchema()); err != nil {
		m.mu.Unlock()
		return err
	}

	newNode := parseNode(value)
	*mod.Node = *newNode

	handler := mod.Handler
	handlerNode := mod.Node

	m.mu.Unlock()

	// Handler BEFORE persist but OUTSIDE lock
	if handler != nil {
		handler(handlerNode)
	}

	// persist + update after handler
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.source.setConfig(jsonConfig); err != nil {
		return err
	}

	m.updateModifiables()
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// REGISTRATION
////////////////////////////////////////////////////////////////////////////////

func (m *Manager) OnInsert(node *Node, handler handler_t) error {
	if node.Type() != Array {
		return errors.New("Node must be array")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	p, err := m.findAndSanitizeNodePath(node)
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
	if node.Type() != Array {
		return errors.New("Node must be array")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	p, err := m.findAndSanitizeNodePath(node)
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
	m.mu.Lock()
	defer m.mu.Unlock()

	p, err := m.findAndSanitizeNodePath(node)
	if err != nil {
		return err
	}

	m.modifiables = append(m.modifiables, modifiable{
		Type:    Replacable,
		Path:    p,
		Node:    node,
		Handler: handler,
	})

	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PATH HELPERS
////////////////////////////////////////////////////////////////////////////////

func (m *Manager) getInsertablePaths() []string { return m.getPaths(Insertable) }
func (m *Manager) getRemovablePaths() []string  { return m.getPaths(Removable) }
func (m *Manager) getReplaceablePaths() []string { return m.getPaths(Replacable) }

func (m *Manager) getPaths(t modifiableType) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := []string{}
	for _, v := range m.modifiables {
		if v.Type == t {
			out = append(out, v.Path)
		}
	}
	return out
}

func (m *Manager) findModifiable(t modifiableType, path string) (*modifiable, error) {
	for i := range m.modifiables {
		if m.modifiables[i].Type == t && m.modifiables[i].Path == path {
			return &m.modifiables[i], nil
		}
	}
	return nil, errors.New("path `" + path + "` not modifiable")
}

func (m *Manager) updateModifiables() {
	// remove invalid ones
	for i := len(m.modifiables) - 1; i >= 0; i-- {
		if m.findNodePath(m.modifiables[i].Node) == "" {
			m.modifiables = append(m.modifiables[:i], m.modifiables[i+1:]...)
		}
	}

	// refresh paths
	for i := range m.modifiables {
		if p := m.findNodePath(m.modifiables[i].Node); p != "" {
			m.modifiables[i].Path = p
		}
	}
}

func (m *Manager) findNodePath(n *Node) string {
	p := findNodePath(m.config, n)
	if p == nil {
		return ""
	}
	return *p
}

func (m *Manager) findAndSanitizeNodePath(n *Node) (string, error) {
	p := m.findNodePath(n)
	if p == "" {
		return "", errors.New("node has no valid path in config")
	}
	return p, nil
}

////////////////////////////////////////////////////////////////////////////////
// HELPERS
////////////////////////////////////////////////////////////////////////////////

func validateJSONAgainstSchema(obj interface{}, schema *string) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	s := string(b)
	return validate(&s, schema)
}
