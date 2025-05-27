package router

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFS struct {
	fs.FS
}

func (m *mockFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func TestFSAdapter(t *testing.T) {
	t.Run("adapts fs.FS", func(t *testing.T) {
		mock := &mockFS{}
		result := fsAdapter(mock)
		assert.Equal(t, mock, result)
	})

	t.Run("adapts http.FileSystem", func(t *testing.T) {
		mock := http.Dir(".")
		result := fsAdapter(mock)
		_, ok := result.(*httpToFS)
		assert.True(t, ok)
	})

	t.Run("panics on invalid type", func(t *testing.T) {
		assert.Panics(t, func() {
			fsAdapter("invalid")
		})
	})
}

func TestConfigHelpers(t *testing.T) {
	t.Run("WebAppConfig", func(t *testing.T) {
		mockFS := &mockFS{}
		cfg := WebAppConfig(mockFS)
		assert.Equal(t, mockFS, cfg.FS)
		assert.Equal(t, "index.html", cfg.IndexFile)
	})

	t.Run("WebAppEnvConfig", func(t *testing.T) {
		t.Setenv("TEST_WEBAPP_PATH", "testdata")
		cfg := WebAppEnvConfig("TEST_WEBAPP_PATH")
		assert.NotNil(t, cfg.FS)
		assert.Equal(t, "index.html", cfg.IndexFile)
	})

	t.Run("StaticSiteConfig", func(t *testing.T) {
		mockFS := &mockFS{}
		cfg := StaticSiteConfig(mockFS)
		assert.Equal(t, mockFS, cfg.FS)
		assert.Equal(t, "index.html", cfg.IndexFile)
	})

	t.Run("StaticSiteEnvConfig", func(t *testing.T) {
		t.Setenv("TEST_SITES_PATH", "testdata")
		cfg := StaticSiteEnvConfig("TEST_SITES_PATH")
		assert.NotNil(t, cfg.FS)
		assert.Equal(t, "index.html", cfg.IndexFile)
	})

	t.Run("PublicFilesConfig", func(t *testing.T) {
		path := "/tmp"
		cfg := PublicFilesConfig(path)
		assert.Equal(t, path, cfg.DirPath)
	})

	t.Run("PublicFilesEnvConfig", func(t *testing.T) {
		t.Setenv("TEST_STATIC_PATH", "/test/path")
		cfg := PublicFilesEnvConfig("TEST_STATIC_PATH")
		assert.Equal(t, "TEST_STATIC_PATH", cfg.EnvVar)
		assert.Equal(t, "/test/path", os.Getenv(cfg.EnvVar))
	})
}

func TestDefaultSetupHelpers(t *testing.T) {
	r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
	require.NoError(t, err)

	t.Run("DefaultWebAppSetup", func(t *testing.T) {
		mockFS := &mockFS{}
		err := DefaultWebAppSetup(r, mockFS)
		assert.NoError(t, err)
	})

	t.Run("DefaultWebAppEnvSetup", func(t *testing.T) {
		t.Setenv("TEST_WEBAPP_PATH", "testdata")
		err := DefaultWebAppEnvSetup(r, "TEST_WEBAPP_PATH")
		assert.NoError(t, err)
	})

	t.Run("MustDefaultWebAppSetup", func(t *testing.T) {
		mockFS := &mockFS{}
		assert.NotPanics(t, func() {
			MustDefaultWebAppSetup(r, mockFS)
		})
	})

	t.Run("MustDefaultWebAppEnvSetup", func(t *testing.T) {
		t.Setenv("TEST_WEBAPP_PATH", "testdata")
		assert.NotPanics(t, func() {
			MustDefaultWebAppEnvSetup(r, "TEST_WEBAPP_PATH")
		})
	})

	t.Run("DefaultStaticSiteSetup", func(t *testing.T) {
		mockFS := &mockFS{}
		err := DefaultStaticSiteSetup(r, mockFS)
		assert.NoError(t, err)
	})

	t.Run("DefaultStaticSiteEnvSetup", func(t *testing.T) {
		t.Setenv("TEST_SITES_PATH", "testdata")
		err := DefaultStaticSiteEnvSetup(r, "TEST_SITES_PATH")
		assert.NoError(t, err)
	})

	t.Run("MustDefaultStaticSiteSetup", func(t *testing.T) {
		mockFS := &mockFS{}
		assert.NotPanics(t, func() {
			MustDefaultStaticSiteSetup(r, mockFS)
		})
	})

	t.Run("MustDefaultStaticSiteEnvSetup", func(t *testing.T) {
		t.Setenv("TEST_SITES_PATH", "testdata")
		assert.NotPanics(t, func() {
			MustDefaultStaticSiteEnvSetup(r, "TEST_SITES_PATH")
		})
	})

	t.Run("DefaultPublicFilesSetup", func(t *testing.T) {
		dir := t.TempDir()
		err := DefaultPublicFilesSetup(r, dir)
		assert.NoError(t, err)
	})

	t.Run("DefaultPublicFilesEnvSetup", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TEST_STATIC_PATH", dir)
		err := DefaultPublicFilesEnvSetup(r, "TEST_STATIC_PATH")
		assert.NoError(t, err)
	})

	t.Run("MustDefaultPublicFilesSetup", func(t *testing.T) {
		dir := t.TempDir()
		assert.NotPanics(t, func() {
			MustDefaultPublicFilesSetup(r, dir)
		})
	})

	t.Run("MustDefaultPublicFilesEnvSetup", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TEST_STATIC_PATH", dir)
		assert.NotPanics(t, func() {
			MustDefaultPublicFilesEnvSetup(r, "TEST_STATIC_PATH")
		})
	})
}

func TestSetupStaticRoutes(t *testing.T) {
	r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
	require.NoError(t, err)

	t.Run("invalid config", func(t *testing.T) {
		err := SetupStaticRoutes(r, StaticConfig{})
		assert.Error(t, err)
	})

	t.Run("with filesystem", func(t *testing.T) {
		mockFS := &mockFS{}
		err := SetupStaticRoutes(r, StaticConfig{
			FS:        mockFS,
			IndexFile: "index.html",
		})
		assert.NoError(t, err)
	})

	t.Run("with directory", func(t *testing.T) {
		dir := t.TempDir()
		err := SetupStaticRoutes(r, StaticConfig{
			DirPath: dir,
		})
		assert.NoError(t, err)
	})

	t.Run("with env var", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TEST_STATIC_PATH", dir)
		err := SetupStaticRoutes(r, StaticConfig{
			EnvVar: "TEST_STATIC_PATH",
		})
		assert.NoError(t, err)
	})

	t.Run("with invalid env var", func(t *testing.T) {
		t.Setenv("TEST_STATIC_PATH", "")
		err := SetupStaticRoutes(r, StaticConfig{
			EnvVar: "TEST_STATIC_PATH",
		})
		assert.Error(t, err)
	})

	t.Run("must version panics on error", func(t *testing.T) {
		assert.Panics(t, func() {
			MustSetupStaticRoutes(r, StaticConfig{})
		})
	})
}

func TestSPAFallback(t *testing.T) {
	r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
	require.NoError(t, err)
	echoRouter := GetRouter(r)

	// Create a test filesystem with index.html
	testFS := os.DirFS("testdata")
	err = os.MkdirAll("testdata", 0755)
	require.NoError(t, err)
	defer os.RemoveAll("testdata")

	err = os.WriteFile("testdata/index.html", []byte("<html></html>"), 0644)
	require.NoError(t, err)

	err = setupSPAFallback(echoRouter, testFS, "index.html")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	echoRouter.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "<html></html>", rec.Body.String())
}

func TestDirectoryFallback(t *testing.T) {
	r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
	require.NoError(t, err)
	echoRouter := GetRouter(r)

	dir := t.TempDir()
	err = os.WriteFile(dir+"/index.html", []byte("<html></html>"), 0644)
	require.NoError(t, err)

	handler := http.FileServer(http.Dir(dir))
	err = setupDirectoryFallback(echoRouter, handler)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	echoRouter.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<html></html>")
}
