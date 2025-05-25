package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPIInfoBuilder(t *testing.T) {
	t.Run("basic info", func(t *testing.T) {
		info := APIInfo().
			Title("Test API").
			Version("1.0.0").
			Description("Test Description").
			TermsOfService("https://example.com/tos").
			Contact("support@example.com", "Support Team").
			License("MIT", "https://opensource.org/licenses/MIT")

		oasInfo := info.toOpenAPI()

		assert.Equal(t, "Test API", oasInfo.Title)
		assert.Equal(t, "1.0.0", oasInfo.Version)
		assert.Equal(t, "Test Description", oasInfo.Description)
		assert.Equal(t, "https://example.com/tos", oasInfo.TermsOfService)

		assert.NotNil(t, oasInfo.Contact)
		assert.Equal(t, "support@example.com", oasInfo.Contact.Email)
		assert.Equal(t, "Support Team", oasInfo.Contact.Name)

		assert.NotNil(t, oasInfo.License)
		assert.Equal(t, "MIT", oasInfo.License.Name)
		assert.Equal(t, "https://opensource.org/licenses/MIT", oasInfo.License.URL)
	})

	t.Run("minimum required fields", func(t *testing.T) {
		info := APIInfo().
			Title("Minimal API").
			Version("0.1.0")

		oasInfo := info.toOpenAPI()

		assert.Equal(t, "Minimal API", oasInfo.Title)
		assert.Equal(t, "0.1.0", oasInfo.Version)
		assert.Empty(t, oasInfo.Description)
		assert.Empty(t, oasInfo.TermsOfService)
		assert.Nil(t, oasInfo.Contact)
		assert.Nil(t, oasInfo.License)
	})

	t.Run("contact without name", func(t *testing.T) {
		info := APIInfo().
			Title("Test").
			Version("1.0").
			Contact("email@test.com", "")

		oasInfo := info.toOpenAPI()
		assert.Equal(t, "email@test.com", oasInfo.Contact.Email)
		assert.Empty(t, oasInfo.Contact.Name)
	})

	t.Run("chaining order", func(t *testing.T) {
		info1 := APIInfo().Title("A").Version("1")
		info2 := APIInfo().Version("1").Title("A")

		assert.Equal(t, info1.toOpenAPI(), info2.toOpenAPI())
	})

	t.Run("nil checks", func(t *testing.T) {
		// Should not panic when converting nil fields
		info := APIInfo().Title("Test").Version("1.0")
		assert.NotPanics(t, func() {
			_ = info.toOpenAPI()
		})
	})

	t.Run("empty builder", func(t *testing.T) {
		info := APIInfo()
		oasInfo := info.toOpenAPI()

		assert.Empty(t, oasInfo.Title)
		assert.Empty(t, oasInfo.Version)
	})

	t.Run("getter methods", func(t *testing.T) {
		info := APIInfo().
			Title("Test API").
			Version("1.0.0").
			Description("Test Description").
			TermsOfService("https://example.com/tos").
			Contact("support@example.com", "Support Team").
			License("MIT", "https://opensource.org/licenses/MIT")

		assert.Equal(t, "Test API", info.GetTitle())
		assert.Equal(t, "1.0.0", info.GetVersion())
		assert.Equal(t, "Test Description", info.GetDescription())
		assert.Equal(t, "https://example.com/tos", info.GetTermsOfService())

		email, name := info.GetContact()
		assert.Equal(t, "support@example.com", email)
		assert.Equal(t, "Support Team", name)

		licenseName, licenseURL := info.GetLicense()
		assert.Equal(t, "MIT", licenseName)
		assert.Equal(t, "https://opensource.org/licenses/MIT", licenseURL)
	})

	t.Run("nil contact and license getters", func(t *testing.T) {
		info := APIInfo().Title("Test").Version("1.0")

		email, name := info.GetContact()
		assert.Empty(t, email)
		assert.Empty(t, name)

		licenseName, licenseURL := info.GetLicense()
		assert.Empty(t, licenseName)
		assert.Empty(t, licenseURL)
	})
}
