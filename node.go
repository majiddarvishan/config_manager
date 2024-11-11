package config

import (
	"errors"
	"strconv"
)

type Node struct {
	value interface{}
}

type NodeType int

const (
	Null NodeType = iota
	Boolean
	Integral
	FloatingPoint
	String
	Object
	Array
)

func (n *Node) Type() NodeType {
	switch n.value.(type) {
	case nil:
		return Null
	case bool:
		return Boolean
	case int64:
		return Integral
	case float64:
		return FloatingPoint
	case string:
		return String
	case map[string]*Node:
		return Object
	case []*Node:
		return Array
	default:
		return Null
	}
}

func (n *Node) get() (interface{}, error) {
	switch v := n.value.(type) {
	case string:
		return v, nil
	case bool:
		return v, nil
		// case int64:
	//     return v, nil
	case int:
		return v, nil
	case float64:
		return v, nil
	case map[string]*Node:
		return v, nil
	case []*Node:
		return v, nil
	default:
		return nil, errors.New("node is not a valid type")
	}
}

// func (n *Node) Get2[T any](param ...string) (string, error) {
//     var tmp T

//     if reflect.TypeOf(tmp).Kind() == string {
//         return n.GetString(param)
//     }

//     // switch v := tmp.(type) {
// 	// case string:
// 	// 	return GetString(param)
// 	// case bool:
// 	// 	return v, nil
// 	// // case int64:
//     // //     return v, nil
//     // case int:
// 	// 	return v, nil
// 	// case float64:
// 	// 	return v, nil
// 	// case map[string]*Node:
// 	// 	return v, nil
// 	// case []*Node:
// 	// 	return v, nil
// 	// default:
// 	// 	return nil, errors.New("Node is not a valid type")
// 	// }

//     return "", errors.New("Node is not a valid type")
// }

func (n *Node) GetString(param ...string) (string, error) {
	if len(param) > 1 {
		return "", errors.New("please input one argument")
	}
	if len(param) == 1 {
		sn, err := n.atString(param[0])
		if err != nil {
			return "", err
		}

		return sn.getString()
	}

	return n.getString()
}

func (n *Node) GetBool(param ...string) (bool, error) {
	if len(param) > 1 {
		return false, errors.New("please input one argument")
	}
	if len(param) == 1 {
		sn, err := n.atString(param[0])
		if err != nil {
			return false, err
		}

		return sn.getBool()
	}

	return n.getBool()
}

func (n *Node) GetInt(param ...string) (int, error) {
	if len(param) > 1 {
		return 0, errors.New("please input one argument")
	}
	if len(param) == 1 {
		sn, err := n.atString(param[0])
		if err != nil {
			return 0, err
		}

		return sn.getInt()
	}

	return n.getInt()
}

func (n *Node) GetFloat(param ...string) (float64, error) {
	if len(param) > 1 {
		return 0, errors.New("please input one argument")
	}
	if len(param) == 1 {
		sn, err := n.atString(param[0])
		if err != nil {
			return 0, err
		}

		return sn.getFloat()
	}

	return n.getFloat()
}

func (n *Node) getString() (string, error) {
	value, err := n.get()
	if err != nil {
		return "", err
	}
	if str, ok := value.(string); ok {
		return str, nil
	}
	return "", errors.New("node is not string")
}

func (n *Node) getBool() (bool, error) {
	value, err := n.get()
	if err != nil {
		return false, err
	}
	if b, ok := value.(bool); ok {
		return b, nil
	}
	return false, errors.New("node is not boolean")
}

func (n *Node) getInt() (int, error) {
	value, err := n.get()
	if err != nil {
		return 0, err
	}

	if i, ok := value.(float64); ok {
		return int(i), nil
	}
	return 0, errors.New("node is not integral")
}

func (n *Node) getFloat() (float64, error) {
	value, err := n.get()
	if err != nil {
		return 0, err
	}
	if f, ok := value.(float64); ok {
		return f, nil
	}
	return 0, errors.New("node is not floating")
}

func (n *Node) GetObject() (map[string]*Node, error) {
	value, err := n.get()
	if err != nil {
		return nil, err
	}
	if obj, ok := value.(map[string]*Node); ok {
		return obj, nil
	}
	return nil, errors.New("node is not object")
}

func (n *Node) GetArray() ([]*Node, error) {
	value, err := n.get()
	if err != nil {
		return nil, err
	}
	if arr, ok := value.([]*Node); ok {
		return arr, nil
	}
	return nil, errors.New("node is not array")
}

func (n *Node) atString(key string) (*Node, error) {
	object, ok := n.value.(map[string]*Node)
	if !ok {
		return nil, errors.New("cannot call at(key) on non object nodes")
	}
	value, ok := object[key]
	if !ok {
		return nil, errors.New("cannot find key `" + key + "` in object Node")
	}
	return value, nil
}

func (n *Node) atInt(index int) (*Node, error) {
	array, ok := n.value.([]*Node)
	if !ok {
		return nil, errors.New("cannot call at(index) on non array nodes")
	}
	if index >= len(array) {
		return nil, errors.New("cannot find index `" + strconv.Itoa(index) + "` in array Node")
	}
	return array[index], nil
}

func (n *Node) At(param interface{}) (*Node, error) {
	if _, ok := param.(int); ok {
		return n.atInt(param.(int))
	} else if _, ok := param.(string); ok {
		return n.atString(param.(string))
	}

	return nil, errors.New("param not int or string")
}
