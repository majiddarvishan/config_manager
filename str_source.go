package config

import (
	"encoding/json"

	"github.com/iancoleman/orderedmap"
)

type StrSource struct {
	configObject *orderedmap.OrderedMap
	config       string
	schema       string
}

func NewStrSource(config, schema string) (*StrSource, error) {
	configMap, err := parseConfig([]byte(config))
	if err != nil {
		return nil, err
	}

	return &StrSource{
		configObject: configMap,
		config:       config,
		schema:       schema,
	}, nil
}

func (s *StrSource) getConfigObject() *orderedmap.OrderedMap {
	return s.configObject
}

func (s *StrSource) getConfig() *string {
	return &s.config
}

func (s *StrSource) getSchema() *string {
	return &s.schema
}

func (s *StrSource) setConfig(conf *orderedmap.OrderedMap) error {
	s.configObject = conf

	configBytes, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}

	s.config = string(configBytes)

    return nil
}
