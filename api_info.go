package router

import "github.com/getkin/kin-openapi/openapi3"

// APIInfoDefinition defines the contract for building OpenAPI info metadata
// APIInfoDefinition defines the contract for building OpenAPI info metadata.
// It provides a fluent interface for setting API documentation properties.
type APIInfoDefinition interface {
	Title(title string) APIInfoDefinition
	GetTitle() string
	Version(version string) APIInfoDefinition
	GetVersion() string
	Description(desc string) APIInfoDefinition
	GetDescription() string
	Contact(email, name string) APIInfoDefinition
	GetContact() (email, name string)
	License(name, url string) APIInfoDefinition
	GetLicense() (name, url string)
	TermsOfService(terms string) APIInfoDefinition
	GetTermsOfService() string
	toOpenAPI() *openapi3.Info
}

// apiInfo implements APIInfoDefinition and holds OpenAPI info metadata.
type apiInfo struct {
	title       string
	version     string
	description string
	contact     *openapi3.Contact
	license     *openapi3.License
	terms       string
}

// APIInfo creates a new APIInfoDefinition builder with default values.
func APIInfo() APIInfoDefinition {
	return &apiInfo{}
}

func (i *apiInfo) Title(title string) APIInfoDefinition {
	i.title = title
	return i
}

func (i *apiInfo) Version(version string) APIInfoDefinition {
	i.version = version
	return i
}

func (i *apiInfo) Description(desc string) APIInfoDefinition {
	i.description = desc
	return i
}

func (i *apiInfo) Contact(email, name string) APIInfoDefinition {
	i.contact = &openapi3.Contact{
		Email: email,
		Name:  name,
	}
	return i
}

func (i *apiInfo) License(name, url string) APIInfoDefinition {
	i.license = &openapi3.License{
		Name: name,
		URL:  url,
	}
	return i
}

func (i *apiInfo) TermsOfService(terms string) APIInfoDefinition {
	i.terms = terms
	return i
}

func (i *apiInfo) GetTitle() string {
	return i.title
}

func (i *apiInfo) GetVersion() string {
	return i.version
}

func (i *apiInfo) GetDescription() string {
	return i.description
}

func (i *apiInfo) GetContact() (email, name string) {
	if i.contact == nil {
		return "", ""
	}
	return i.contact.Email, i.contact.Name
}

func (i *apiInfo) GetLicense() (name, url string) {
	if i.license == nil {
		return "", ""
	}
	return i.license.Name, i.license.URL
}

func (i *apiInfo) GetTermsOfService() string {
	return i.terms
}

func (i *apiInfo) toOpenAPI() *openapi3.Info {
	return &openapi3.Info{
		Title:          i.title,
		Version:        i.version,
		Description:    i.description,
		Contact:        i.contact,
		License:        i.license,
		TermsOfService: i.terms,
	}
}
