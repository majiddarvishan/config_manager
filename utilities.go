package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/iancoleman/orderedmap"
)

func jsonSetByPath(jsonMap *orderedmap.OrderedMap, path string, value interface{}) bool {
	splited_path := strings.Split(path, "/")

	foundMap := *jsonMap

	for i := 0; i < len(splited_path)-1; {
		if len(splited_path[i]) != 0 {
			found, present := foundMap.Get(splited_path[i])
			if !present {
				return false
			}

			k := reflect.TypeOf(found).Kind()
			switch k {
			case reflect.Struct:
				foundMap = found.(orderedmap.OrderedMap)
			case reflect.Slice:
				s := found.([]interface{})
				index, err := strconv.ParseInt(splited_path[i+1], 10, 32)
				if err != nil {
					fmt.Printf("path '%v' is not valid", path)
					return false
				}
				found = s[index]
				i++
				foundMap = found.(orderedmap.OrderedMap)
			default:
				fmt.Printf("type '%v' is not valid", k)
				return false
			}
		}
		i++
	}

	foundMap.Set(splited_path[len(splited_path)-1], value)

	return true
}

func jsonRemoveByPath(jsonMap *orderedmap.OrderedMap, path string, index int) bool {
	splited_path := strings.Split(path, "/")

	foundMap := *jsonMap

	for i := 0; i < len(splited_path)-1; {
		if len(splited_path[i]) != 0 {
			found, present := foundMap.Get(splited_path[i])
			if !present {
				return false
			}

			k := reflect.TypeOf(found).Kind()
			switch k {
			case reflect.Struct:
				foundMap = found.(orderedmap.OrderedMap)
			case reflect.Slice:
				s := found.([]interface{})
				index, err := strconv.ParseInt(splited_path[i+1], 10, 32)
				if err != nil {
					fmt.Printf("path '%v' is not valid", path)
					return false
				}
				found = s[index]
				i++
				foundMap = found.(orderedmap.OrderedMap)
			default:
				fmt.Printf("type '%v' is not valid", k)
				return false
			}
		}
		i++
	}

	found_list, present := foundMap.Get(splited_path[len(splited_path)-1])
	if !present {
		return false
	}

	if index >= len(found_list.([]interface{})) {
		// fmt.Println("Index is out of bounds")
		return false
	}

	found_list = append(found_list.([]interface{})[:index], found_list.([]interface{})[index+1:]...)
	foundMap.Set(splited_path[len(splited_path)-1], found_list)

	return true
}

func jsonInsertByPath(jsonMap *orderedmap.OrderedMap, path string, index int, value interface{}) bool {
	splited_path := strings.Split(path, "/")

	foundMap := *jsonMap

	for i := 0; i < len(splited_path)-1; {
		if len(splited_path[i]) != 0 {
			found, present := foundMap.Get(splited_path[i])
			if !present {
				return false
			}

			k := reflect.TypeOf(found).Kind()
			switch k {
			case reflect.Struct:
				foundMap = found.(orderedmap.OrderedMap)
			case reflect.Slice:
				s := found.([]interface{})
				index, err := strconv.ParseInt(splited_path[i+1], 10, 32)
				if err != nil {
					fmt.Printf("path '%v' is not valid", path)
					return false
				}
				found = s[index]
				i++
				foundMap = found.(orderedmap.OrderedMap)
			default:
				fmt.Printf("type '%v' is not valid", k)
				return false
			}
		}
		i++
	}

	found_list, present := foundMap.Get(splited_path[len(splited_path)-1])
	if !present {
		return false
	}

	if index > len(found_list.([]interface{})) {
		// fmt.Println("Index is out of bounds")
		return false
	}

	found_list = append(found_list.([]interface{})[:index], append([]interface{}{value}, found_list.([]interface{})[index:]...)...)
	foundMap.Set(splited_path[len(splited_path)-1], found_list)

	return true
}

func findNodePath(parentNode *Node, desiredNode *Node) *string {
	if parentNode == desiredNode {
		path := ""
		return &path
	}
	if parentNode.Type() == Array {
		index := 0
		arr, err := parentNode.GetArray()
		if err == nil {
			for _, innerNode := range arr {
				subPath := findNodePath(innerNode, desiredNode)
				if subPath != nil {
					path := "/" + strconv.Itoa(index) + *subPath
					return &path
				}
				index++
			}
		}
	} else if parentNode.Type() == Object {
		obj, err := parentNode.GetObject()
		if err == nil {
			for key, innerNode := range obj {
				subPath := findNodePath(innerNode, desiredNode)
				if subPath != nil {
					path := "/" + key + *subPath
					return &path
				}
			}
		}
	}

	return nil
}

// func jsonToNode(jsonData []byte) (*Node, error) {
// 	var jsonMap map[string]interface{}
// 	err := json.Unmarshal(jsonData, &jsonMap)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return parseNode(jsonMap), nil
// }

func parseNode(value any) *Node {
	node := &Node{}

	if m, ok := value.(*map[string]interface{}); ok {
		obj := make(map[string]*Node)
		for k, v := range *m {
			obj[k] = parseNode(v)
		}
		node.value = obj
	} else if m, ok := value.(map[string]interface{}); ok {
		obj := make(map[string]*Node)
		for k, v := range m {
			obj[k] = parseNode(v)
		}
		node.value = obj
	} else if m, ok := value.(*orderedmap.OrderedMap); ok {
		obj := make(map[string]*Node)

		keys := m.Keys()
		for _, k := range keys {
			v, _ := m.Get(k)
			obj[k] = parseNode(v)
		}

		node.value = obj
	} else if m, ok := value.(orderedmap.OrderedMap); ok {
		obj := make(map[string]*Node)

		keys := m.Keys()
		for _, k := range keys {
			v, _ := m.Get(k)
			obj[k] = parseNode(v)
		}

		node.value = obj
	} else if m, ok := value.(*[]interface{}); ok {
		var arr []*Node
		for _, v := range *m {
			arr = append(arr, parseNode(v))
		}
		node.value = arr
	} else if m, ok := value.([]interface{}); ok {
		var arr []*Node
		for _, v := range m {
			arr = append(arr, parseNode(v))
		}
		node.value = arr
	} else if m, ok := value.(string); ok {
		node.value = m
	} else if m, ok := value.(int); ok {
		node.value = m
	} else if m, ok := value.(float64); ok {
		node.value = m
	} else if m, ok := value.(bool); ok {
		node.value = m
	} else {
		node.value = ""
	}

	return node
}

// Serialize + deserialize to deep copy values
func Clone(om *orderedmap.OrderedMap) *orderedmap.OrderedMap {
    data, err := json.Marshal(om)
	if err != nil {
        fmt.Println(err)
		return nil
	}

    clone := orderedmap.New()
	if err := json.Unmarshal(data, &clone); err != nil {
		fmt.Println(err)
		return nil
	}

	return clone
}
