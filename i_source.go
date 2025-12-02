package config

import "github.com/iancoleman/orderedmap"

type ISource interface {
	getConfigObject() *orderedmap.OrderedMap
	getConfig() *string
	getSchema() *string
	setConfig(*orderedmap.OrderedMap) error
}
