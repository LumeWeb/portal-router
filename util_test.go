package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSubdomain(t *testing.T) {
	t.Run("valid subdomain", func(t *testing.T) {
		assert.True(t, IsSubdomain("sub.example.com"))
	})

	t.Run("valid domain", func(t *testing.T) {
		assert.False(t, IsSubdomain("example.com"))
	})

	t.Run("valid domain with multiple labels", func(t *testing.T) {
		assert.False(t, IsSubdomain("example.co.uk"))
	})

	t.Run("valid subdomain with multiple labels", func(t *testing.T) {
		assert.True(t, IsSubdomain("sub.example.co.uk"))
	})

	t.Run("invalid domain", func(t *testing.T) {
		assert.False(t, IsSubdomain("invalid"))
	})

	t.Run("empty domain", func(t *testing.T) {
		assert.False(t, IsSubdomain(""))
	})

	t.Run("domain with only public suffix", func(t *testing.T) {
		assert.False(t, IsSubdomain("com"))
	})

	t.Run("domain with public suffix and single label is not a subdomain", func(t *testing.T) {
		assert.False(t, IsSubdomain("sub.com"))
	})

	t.Run("not ICANN managed", func(t *testing.T) {
		assert.False(t, IsSubdomain("example.localhost"))
	})
}
