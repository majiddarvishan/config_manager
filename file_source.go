package config

import (
	"encoding/json"
	"os"

	"github.com/iancoleman/orderedmap"
)

func parseConfig(config []byte) (*orderedmap.OrderedMap, error) {
	var result = orderedmap.New()

	err := json.Unmarshal([]byte(config), &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type FileSource struct {
	configPath   string
	configObject *orderedmap.OrderedMap
	config       string
	schema       string
}

func NewFileSource(configPath string, schema string) (*FileSource, error) {
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	config, err := parseConfig(configBytes)
	if err != nil {
		return nil, err
	}

	return &FileSource{
		configPath:   configPath,
		configObject: config,
		config:       string(configBytes),
		schema:       schema,
	}, nil
}

func (fs *FileSource) getConfigObject() *orderedmap.OrderedMap {
	return fs.configObject
}

func (fs *FileSource) getConfig() *string {
	return &fs.config
}

func (fs *FileSource) getSchema() *string {
	return &fs.schema
}

func (fs *FileSource) setConfig(conf *orderedmap.OrderedMap) error {
	configBytes, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}

	c := string(configBytes)
	err = validate(&c, &fs.schema)
	if err != nil {
		return err
	}

	err = os.WriteFile(fs.configPath, configBytes, os.ModePerm)
	if err != nil {
		return err
	}

	fs.configObject = conf
	fs.config = string(configBytes)

    return nil
}
