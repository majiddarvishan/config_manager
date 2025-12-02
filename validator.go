package config

import (
	"errors"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

func validate(conf, schema *string) error {
	loadedSchema := gojsonschema.NewBytesLoader([]byte(*schema))
	documentLoader := gojsonschema.NewBytesLoader([]byte(*conf))

	result, err := gojsonschema.Validate(loadedSchema, documentLoader)
	if err != nil {
        return err
	}

	// Check the validity of the result and throw a message is the document is valid or if it's not with errors.
	if !result.Valid() {
        var sb strings.Builder
        for i, desc := range result.Errors() {
            if i > 0 {
                sb.WriteString("\n")   // add separator before every item except the first
            }
            sb.WriteString(desc.String())
        }

        return errors.New(sb.String())

        // err_desc := sb.String()

        // err_desc := ""
        // for _, desc := range result.Errors() {
        //     err_desc += desc.String()
        //     err_desc += "\n"
		// }
        // return errors.New(err_desc)
	}

	return nil
}