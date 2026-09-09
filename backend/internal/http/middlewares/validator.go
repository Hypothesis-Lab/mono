package middlewares

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
)

// validationMessage maps an OpenAPI request validation failure to a
// user-facing message, deriving the field and constraint from the
// error's schema metadata.
func validationMessage(err error, status int) string {
	var reqErr *openapi3filter.RequestError
	if !errors.As(err, &reqErr) || reqErr.Parameter != nil {
		return http.StatusText(status)
	}

	if strings.HasPrefix(reqErr.Reason, "header Content-Type has unexpected value") {
		return "unsupported content type"
	}

	if errors.Is(err, openapi3filter.ErrInvalidRequired) {
		return "request body is required"
	}

	var parseErr *openapi3filter.ParseError
	if errors.As(err, &parseErr) {
		return "malformed request body"
	}

	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) {
		field := "value"
		path := schemaErr.JSONPointer()
		for i := len(path) - 1; i >= 0; i-- {
			if _, parseErr := strconv.Atoi(path[i]); parseErr == nil {
				continue
			}

			field = path[i]
			break
		}

		switch schemaErr.SchemaField {
		case "required":
			return fmt.Sprintf("%s is required", field)
		case "minLength":
			return fmt.Sprintf("%s must be at least %d characters long", field, schemaErr.Schema.MinLength)
		case "maxLength":
			return fmt.Sprintf("%s must be at most %d characters long", field, *schemaErr.Schema.MaxLength)
		case "minItems":
			return fmt.Sprintf("%s must have at least %d items", field, schemaErr.Schema.MinItems)
		case "maxItems":
			return fmt.Sprintf("%s must have at most %d items", field, *schemaErr.Schema.MaxItems)
		case "minimum":
			return fmt.Sprintf("%s must be greater than or equal to %v", field, *schemaErr.Schema.Min)
		case "maximum":
			return fmt.Sprintf("%s must be less than or equal to %v", field, *schemaErr.Schema.Max)
		case "enum":
			values := make([]string, 0, len(schemaErr.Schema.Enum))
			for _, v := range schemaErr.Schema.Enum {
				values = append(values, fmt.Sprintf("%v", v))
			}

			return fmt.Sprintf("%s must be one of %s", field, strings.Join(values, ", "))
		case "type":
			return fmt.Sprintf("%s has invalid type", field)
		}
	}

	return http.StatusText(status)
}
