package config

import (
	"errors"

	"github.com/xeipuuv/gojsonschema"
)

func validate(conf, schema *string) error {
	loadedSchema := gojsonschema.NewBytesLoader([]byte(*schema))
	documentLoader := gojsonschema.NewBytesLoader([]byte(*conf))

	result, err := gojsonschema.Validate(loadedSchema, documentLoader)
	if err != nil {
        return errors.New("There was a problem on validating.\n" + err.Error())
	}

	// Check the validity of the result and throw a message is the document is valid or if it's not with errors.
	if !result.Valid() {
        err_desc := ""
        for _, desc := range result.Errors() {
            err_desc += desc.String()
            err_desc += "\n"
		}
        return errors.New(err_desc)
	}

	return nil
}