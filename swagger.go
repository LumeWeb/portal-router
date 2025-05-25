package router

import (
	swagger "go.lumeweb.com/gswagger"
	"strings"
)

// SwaggerOption defines a function type for modifying swagger.Definitions.
// Used to apply customizations to OpenAPI/Swagger documentation.
type SwaggerOption func(*swagger.Definitions)

// FieldSchema defines the interface for providing field-level schema information
// used to generate sorting and filtering documentation.
type FieldSchema interface {
	SortableFields() []string
	FilterOperators() map[string][]string // field -> []operator
}

// WithSchema creates a SwaggerOption that adds sorting and filtering parameters
// based on the provided FieldSchema implementation.
func WithSchema(schema FieldSchema) SwaggerOption {
	return func(d *swagger.Definitions) {
		// Add sort parameters
		sortFields := schema.SortableFields()
		d.Querystring["_sort"] = swagger.Parameter{
			Description: "Sort by fields: " + strings.Join(sortFields, ", "),
			Schema: &swagger.Schema{
				Value: map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
						"enum": sortFields,
					},
				},
			},
		}

		d.Querystring["_order"] = swagger.Parameter{
			Description: "Sort direction",
			Schema: &swagger.Schema{
				Value: map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
						"enum": []string{"asc", "desc"},
					},
				},
			},
		}

		// Add filter parameters
		operators := schema.FilterOperators()
		for field, ops := range operators {
			d.Querystring[field] = swagger.Parameter{
				Description: "Filter operators: " + strings.Join(ops, ", "),
				Schema: &swagger.Schema{
					Value: map[string]any{
						"type":       "object",
						"properties": createOperatorSchemas(ops),
					},
				},
			}
		}
	}
}

// SchemaProvider defines an interface for providing schema information
// based on type.
type SchemaProvider interface {
	ForType(any) FieldSchema
}

// createOperatorSchemas generates schema definitions for filter operators.
// Returns a map of operator names to their schema definitions.
func createOperatorSchemas(ops []string) map[string]any {
	schemas := make(map[string]any)
	for _, op := range ops {
		schemas[op] = map[string]any{
			"type":        "string", // Actual type resolved by provider
			"description": operatorDocs[op],
		}
	}
	return schemas
}

var operatorDocs = map[string]string{
	"eq":           "Equal to",
	"ne":           "Not equal to",
	"neq":          "Not equal to (alias for ne)",
	"lt":           "Less than",
	"gt":           "Greater than",
	"lte":          "Less than or equal to",
	"gte":          "Greater than or equal to",
	"in":           "Value is in the specified array",
	"nin":          "Value is not in the specified array",
	"contains":     "Case-insensitive contains",
	"containss":    "Case-sensitive contains",
	"ncontains":    "Case-insensitive does not contain",
	"ncontainss":   "Case-sensitive does not contain",
	"between":      "Value is between two values (inclusive)",
	"nbetween":     "Value is not between two values",
	"null":         "Value is null",
	"nnull":        "Value is not null",
	"startswith":   "Case-insensitive starts with",
	"startswiths":  "Case-sensitive starts with",
	"nstartswith":  "Case-insensitive does not start with",
	"nstartswiths": "Case-sensitive does not start with",
	"endswith":     "Case-insensitive ends with",
	"endswiths":    "Case-sensitive ends with",
	"nendswith":    "Case-insensitive does not end with",
	"nendswiths":   "Case-sensitive does not end with",
	"ina":          "Array contains any of the specified values",
	"nina":         "Array does not contain any of the specified values",
	"like":         "Case-insensitive contains (alias for contains)",
}
