package router

import (
	swagger "go.lumeweb.com/gswagger"
	"net/http"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// Shared base for all endpoints
// baseDefinition returns a swagger.Definitions with common response schemas
// pre-configured for success and error cases.
func baseDefinition() swagger.Definitions {
	def := swagger.Definitions{
		Responses:   make(map[int]swagger.ContentValue),
		PathParams:  make(swagger.ParameterValue),
		Querystring: make(swagger.ParameterValue),
		Headers:     make(swagger.ParameterValue),
		Cookies:     make(swagger.ParameterValue),
	}
	def.Responses = map[int]swagger.ContentValue{
		http.StatusOK:                  defaultSuccessResponse(),
		http.StatusBadRequest:          badRequestResponse(),
		http.StatusUnauthorized:        unauthorizedResponse(),
		http.StatusForbidden:           forbiddenResponse(),
		http.StatusNotFound:            notFoundResponse(),
		http.StatusUnprocessableEntity: validationFailedResponse(),
		http.StatusInternalServerError: internalServerErrorResponse(),
	}
	return def
}

// applySwaggerDefaults applies default Swagger responses to ensure consistent
// documentation across all routes. Adds 200 OK if not already present.
func applySwaggerDefaults(def *swagger.Definitions) {
	if def.Responses == nil {
		def.Responses = make(map[int]swagger.ContentValue)
	}
	if _, hasNoDefaultMarker := def.Responses[-1]; hasNoDefaultMarker {
		return
	}
	if _, ok := def.Responses[http.StatusOK]; !ok {
		def.Responses[http.StatusOK] = defaultSuccessResponse()
	}
}

// clearSwaggerMarkers removes internal markers from swagger definitions.
// Called in RegisterRoutes before passing swagger to the router.
func clearSwaggerMarkers(def *swagger.Definitions) {
	if def.Responses != nil {
		delete(def.Responses, -1)
	}
}

// defaultSuccessResponse returns the standard success response structure
func defaultSuccessResponse() swagger.ContentValue {
	return swagger.ContentValue{
		Description: "Success",
		Content: swagger.Content{
			MediaTypeJSON: {
				Value: ErrorResponse{Message: ""}, // Empty message indicates success
			},
		},
	}
}

// badRequestResponse returns the standard 400 Bad Request response
func badRequestResponse() swagger.ContentValue {
	return swagger.ContentValue{
		Description: "Bad Request",
		Content: swagger.Content{
			MediaTypeJSON: {
				Value: ErrorResponse{Message: "Bad Request"},
			},
		},
	}
}

// unauthorizedResponse returns the standard 401 Unauthorized response
func unauthorizedResponse() swagger.ContentValue {
	return swagger.ContentValue{
		Description: "Unauthorized",
		Content: swagger.Content{
			MediaTypeJSON: {
				Value: ErrorResponse{Message: "Unauthorized"},
			},
		},
	}
}

// forbiddenResponse returns the standard 403 Forbidden response
func forbiddenResponse() swagger.ContentValue {
	return swagger.ContentValue{
		Description: "Forbidden",
		Content: swagger.Content{
			MediaTypeJSON: {
				Value: ErrorResponse{Message: "Forbidden"},
			},
		},
	}
}

// notFoundResponse returns the standard 404 Not Found response
func notFoundResponse() swagger.ContentValue {
	return swagger.ContentValue{
		Description: "Not Found",
		Content: swagger.Content{
			MediaTypeJSON: {
				Value: ErrorResponse{Message: "Not Found"},
			},
		},
	}
}

// validationFailedResponse returns the standard 422 Unprocessable Entity response
func validationFailedResponse() swagger.ContentValue {
	return swagger.ContentValue{
		Description: "Validation Failed",
		Content: swagger.Content{
			MediaTypeJSON: {
				Value: ErrorResponse{Message: "Validation Failed"},
			},
		},
	}
}

// IsSubdomain checks if the given domain is a subdomain of its public suffix.
// Returns true if domain has parts before the public suffix, false otherwise.
func IsSubdomain(domain string) bool {
	if domain == "" {
		return false
	}

	// Get public suffix (e.g. "co.uk" for "example.co.uk")
	suffix, icann := publicsuffix.PublicSuffix(domain)

	// Non-ICANN managed domains are never considered subdomains
	if !icann {
		return false
	}

	// If domain equals the public suffix, it's not a subdomain
	if domain == suffix {
		return false
	}

	// Remove the suffix from the domain
	base := strings.TrimSuffix(domain, "."+suffix)

	// If the base is empty, it's not a subdomain (e.g., "co.uk")
	if base == "" {
		return false
	}

	// If the base contains a dot, it's a subdomain
	if strings.Contains(base, ".") {
		return true
	}

	return false
}

// internalServerErrorResponse returns the standard 500 Internal Server Error response
func internalServerErrorResponse() swagger.ContentValue {
	return swagger.ContentValue{
		Description: "Internal Server Error",
		Content: swagger.Content{
			MediaTypeJSON: {
				Value: ErrorResponse{Message: "Internal Server Error"},
			},
		},
	}
}

// applySwaggerOpts applies a set of SwaggerOptions to a swagger.Definitions,
// returning the modified definitions. This is the centralized place for applying
// Swagger-specific options.
func applySwaggerOpts(d swagger.Definitions, access string, opts []SwaggerOption) swagger.Definitions {
	// Make shallow copy of the definition
	result := d

	// Ensure maps are initialized if nil
	if result.Responses == nil {
		result.Responses = make(map[int]swagger.ContentValue)
	}
	if result.PathParams == nil {
		result.PathParams = make(swagger.ParameterValue)
	}
	if result.Querystring == nil {
		result.Querystring = make(swagger.ParameterValue)
	}
	if result.Headers == nil {
		result.Headers = make(swagger.ParameterValue)
	}
	if result.Cookies == nil {
		result.Cookies = make(swagger.ParameterValue)
	}
	if result.Parameters == nil {
		result.Parameters = make(map[string]swagger.ParameterDefinition)
	}

	// Safely apply options if they exist
	if opts != nil {
		for _, opt := range opts {
			if opt != nil {
				opt(&result, access)
			}
		}
	}

	return result
}
