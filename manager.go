package config

import (
	"errors"
	"log"
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
	Handler *handler_t
}

type Manager struct {
	source      ISource
	config      *Node
	modifiables []modifiable
}

func NewManager(source ISource) *Manager {
	m := &Manager{
		source:      source,
		config:      parseNode(source.getConfigObject()),
		modifiables: make([]modifiable, 0),
	}

	err := validate(source.getConfig(), source.getSchema())
	if err != nil {
		log.Println(err)
		return nil
	}

	return m
}

func (m *Manager) Config() *Node {
	return m.config
}

func (m *Manager) Source() ISource {
	return m.source
}

func (m *Manager) insert(path string, index int, value interface{}) {
	mod, err := m.findModifiable(Insertable, path)
	if err != nil {
		panic(err)
	}

	jsonConfig := m.source.getConfigObject()
	ok := jsonInsertByPath(jsonConfig, path, index, value)
	if !ok {
		panic("should be there!")
	}

	//todo
	// validator.Validate(jsonConfig)

	insertingNode := parseNode(value)
	backupArray, err := mod.Node.GetArray()
	if err != nil {
		panic(err)
	}

	array := make([]*Node, len(backupArray))
	copy(array, backupArray)

	array = append(array[:index], append([]*Node{insertingNode}, array[index:]...)...)
	*(mod.Node) = Node{array}

	// try
	// {
	(*mod.Handler)(insertingNode)
	// }
	// catch (...)
	// {
	//     *(mod.Node) = Node{ backup_array };
	//     throw;
	// }

	m.source.setConfig(jsonConfig)
	m.updateModifiables()
}

func (m *Manager) remove(path string, index int) {
	mod, err := m.findModifiable(Removable, path)
	if err != nil {
		panic(err)
	}

	jsonConfig := m.source.getConfigObject()
	ok := jsonRemoveByPath(jsonConfig, path, index)
	if !ok {
		panic("should be there!")
	}

	//todo
	// validator.validate(jsonConfig)

	backupArray, err := mod.Node.GetArray()
	if err != nil {
		panic(err)
	}

	array := make([]*Node, len(backupArray))
	copy(array, backupArray)
	removingNode := array[index]
	array = append(array[:index], array[index+1:]...)
	*(mod.Node) = Node{array}

	// try
	// {
	(*mod.Handler)(removingNode)
	// }
	// catch (...)
	// {
	//     *(mod.Node) = Node{ backup_array };
	//     throw;
	// }

	m.source.setConfig(jsonConfig)
	m.updateModifiables()
}

func (m *Manager) replace(path string, value interface{}) {
	mod, err := m.findModifiable(Replacable, path)
	if err != nil {
		panic(err)
	}

	jsonConfig := m.source.getConfigObject()
	ok := jsonSetByPath(jsonConfig, path, value)
	if !ok {
		panic("should be there!")
	}

	//todo
	// validator.Validate(jsonConfig)
	newNode := parseNode(value)
	// backupNode := *mod.Node
	mod.Node = newNode

	// try
	// {
	(*mod.Handler)(mod.Node)
	// }
	// catch (...)
	// {
	// *mod.Node = backup_node;
	//     throw;
	// }

	m.source.setConfig(jsonConfig)
	m.updateModifiables()
}

func (m *Manager) OnInsert(node *Node, handler handler_t) error {
	if node.Type() != Array {
		return errors.New("Node type must be an array")
	}
	path, err := m.findAndSanitizeNodePath(node)
	if err == nil {
		m.modifiables = append(m.modifiables, modifiable{
			Type:    Insertable,
			Path:    path,
			Node:    node,
			Handler: &handler,
		})
		return nil
	}

	return err
}

func (m *Manager) OnRemove(node *Node, handler handler_t) error {
	if node.Type() != Array {
		return errors.New("Node type must be an array")
	}

	path, err := m.findAndSanitizeNodePath(node)
	if err == nil {
		// obs := newObserver(m, handler)
		// defer obs.deregister()
		m.modifiables = append(m.modifiables, modifiable{
			Type:    Removable,
			Path:    path,
			Node:    node,
			Handler: &handler,
		})
		return nil
	}

	return err
}

func (m *Manager) OnReplace(node *Node, handler handler_t) error {
	path, err := m.findAndSanitizeNodePath(node)
	if err == nil {
		// obs := newObserver(m, handler)
		// defer obs.deregister()
		m.modifiables = append(m.modifiables, modifiable{
			Type:    Replacable,
			Path:    path,
			Node:    node,
			Handler: &handler,
		})

		return nil
	}

	return err
}

func (m *Manager) getInsertablePaths() []string {
	return m.getModifiablePaths(Insertable)
}

func (m *Manager) getRemovablePaths() []string {
	return m.getModifiablePaths(Removable)
}

func (m *Manager) getReplaceablePaths() []string {
	return m.getModifiablePaths(Replacable)
}

func (m *Manager) getModifiablePaths(typ modifiableType) []string {
	res := []string{}
	for _, mod := range m.modifiables {
		if mod.Type == typ {
			res = append(res, mod.Path)
		}
	}
	return res
}

func (m *Manager) findModifiable(typ modifiableType, path string) (*modifiable, error) {
	for _, mod := range m.modifiables {
		if mod.Type == typ && mod.Path == path {
			return &mod, nil
		}
	}
	return nil, errors.New("The path `" + path + "` does not refer to a modifiable Node")
}

func (m *Manager) updateModifiables() {
	for i := len(m.modifiables) - 1; i >= 0; i-- {
		if m.findNodePath(m.modifiables[i].Node) == "" {
			m.modifiables = append(m.modifiables[:i], m.modifiables[i+1:]...)
		}
	}

	for i := len(m.modifiables) - 1; i >= 0; i-- {
		if np := m.findNodePath(m.modifiables[i].Node); np != "" {
			m.modifiables[i].Path = np
		}
	}
}

func (m *Manager) findAndSanitizeNodePath(n *Node) (string, error) {
	path := m.findNodePath(n)
	if path == "" {
		return "", errors.New("cannot find the Node in the config, cannot observe on a disjointed Node")
	}
	return path, nil
}

func (m *Manager) findNodePath(n *Node) string {
	np := findNodePath(m.config, n)
	if np != nil {
		return *np
	}
	return ""
}
