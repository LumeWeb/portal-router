package router

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// httpToFS wraps an http.FileSystem to implement fs.FS
type httpToFS struct {
	fs http.FileSystem
}

// Open implements fs.FS.Open
func (h httpToFS) Open(name string) (fs.File, error) {
	return h.fs.Open(name)
}

// fsAdapter adapts between fs.FS and http.FileSystem interfaces
func fsAdapter(fsys any) fs.FS {
	switch v := fsys.(type) {
	case fs.FS:
		return v
	case http.FileSystem:
		return &httpToFS{fs: v}
	default:
		panic("fsAdapter: unsupported filesystem type")
	}
}

// Predefined configurations for common static file setups

// StaticConfigWithFS returns a StaticConfig with the given filesystem and SPA fallback to index.html.
// The fsys parameter can be either:
// - fs.FS (for embedded files)
// - http.FileSystem (for directory-based files)
// - nil (will return empty StaticConfig)
func StaticConfigWithFS(fsys any) StaticConfig {
	if fsys == nil {
		return StaticConfig{}
	}
	return StaticConfig{
		FS:        fsAdapter(fsys),
		IndexFile: "index.html",
	}
}

// StaticConfigFromEnv returns a StaticConfig that reads the filesystem path from an environment variable.
// Uses os.DirFS to create the filesystem from the directory path.
// Returns empty StaticConfig if envVar is not set or empty.
func StaticConfigFromEnv(envVar string) StaticConfig {
	path := os.Getenv(envVar)
	if path == "" {
		return StaticConfig{}
	}
	return StaticConfig{
		FS:        os.DirFS(path),
		IndexFile: "index.html",
	}
}

// PublicFilesConfig returns a StaticConfig for serving public files from disk
func PublicFilesConfig(dirPath string) StaticConfig {
	return StaticConfig{
		DirPath: dirPath,
	}
}

// PublicFilesEnvConfig returns a StaticConfig that reads the directory path from an environment variable
func PublicFilesEnvConfig(envVar string) StaticConfig {
	path := os.Getenv(envVar)
	return StaticConfig{
		DirPath: path,
		EnvVar:  envVar,
	}
}

// DefaultStaticSetup is a one-liner to setup static file serving with the given filesystem
func DefaultStaticSetup(r Router, fsys any) error {
	return SetupStaticRoutes(r, StaticConfigWithFS(fsys))
}

// DefaultStaticEnvSetup is a one-liner to setup static file serving from an environment variable
func DefaultStaticEnvSetup(r Router, envVar string) error {
	return SetupStaticRoutes(r, StaticConfigFromEnv(envVar))
}

// MustDefaultStaticSetup is a panic-on-error version of DefaultStaticSetup
func MustDefaultStaticSetup(r Router, fsys any) {
	MustSetupStaticRoutes(r, StaticConfigWithFS(fsys))
}

// MustDefaultStaticEnvSetup is a panic-on-error version of DefaultStaticEnvSetup
func MustDefaultStaticEnvSetup(r Router, envVar string) {
	MustSetupStaticRoutes(r, StaticConfigFromEnv(envVar))
}

// DefaultPublicFilesSetup is a one-liner to setup public file serving
func DefaultPublicFilesSetup(r Router, dirPath string) error {
	return SetupStaticRoutes(r, PublicFilesConfig(dirPath))
}

// DefaultPublicFilesEnvSetup is a one-liner to setup public files from env var
func DefaultPublicFilesEnvSetup(r Router, envVar string) error {
	return SetupStaticRoutes(r, PublicFilesEnvConfig(envVar))
}

// MustDefaultPublicFilesSetup is a panic-on-error version of DefaultPublicFilesSetup
func MustDefaultPublicFilesSetup(r Router, dirPath string) {
	MustSetupStaticRoutes(r, PublicFilesConfig(dirPath))
}

// MustDefaultPublicFilesEnvSetup is a panic-on-error version of DefaultPublicFilesEnvSetup
func MustDefaultPublicFilesEnvSetup(r Router, envVar string) {
	MustSetupStaticRoutes(r, PublicFilesEnvConfig(envVar))
}

// StaticConfig holds configuration for static file serving
type StaticConfig struct {
	DirPath        string       // Path to directory for static files
	EnvVar         string       // Environment variable containing path to directory
	DefaultHandler http.Handler // Fallback handler if DirPath is empty
	FS             fs.FS        // Filesystem for embedded static files
	IndexFile      string       // Name of index file for SPA fallback (required when using FS)
}

// SetupStaticRoutes configures static file serving and SPA fallback routes.
// It supports both regular directories and embedded filesystems.
func SetupStaticRoutes(r Router, cfg StaticConfig) error {
	// Validate configuration
	if cfg.DirPath == "" && cfg.DefaultHandler == nil && cfg.FS == nil && cfg.EnvVar == "" {
		return errors.New("must provide either DirPath, DefaultHandler, FS or EnvVar")
	}
	if cfg.FS != nil && cfg.IndexFile == "" {
		return errors.New("IndexFile is required when using FS")
	}

	echoRouter := GetRouter(r)

	// Handle embedded filesystem case
	if cfg.FS != nil {
		echoRouter.StaticFS("/assets", fsAdapter(cfg.FS))
		return setupSPAFallback(echoRouter, cfg.FS, cfg.IndexFile)
	}

	// Handle regular directory case
	var handler http.Handler
	dirPath := cfg.DirPath
	if dirPath == "" && cfg.EnvVar != "" {
		dirPath = os.Getenv(cfg.EnvVar)
	}

	if dirPath != "" {
		handler = http.FileServer(http.Dir(dirPath))
		echoRouter.Static("/assets/*", dirPath)
	} else if cfg.DefaultHandler != nil {
		handler = cfg.DefaultHandler
	} else {
		return errors.New("must provide either valid DirPath (from EnvVar or directly), DefaultHandler or FS")
	}
	return setupDirectoryFallback(echoRouter, handler)
}

// setupSPAFallback configures the SPA fallback route for embedded filesystems
func setupSPAFallback(echoRouter *echo.Echo, fsys fs.FS, indexFile string) error {
	echoRouter.GET("/*", func(c echo.Context) error {
		if strings.HasPrefix(c.Request().URL.Path, "/api/") {
			return echo.ErrNotFound
		}

		file, err := fsys.Open(indexFile)
		if err != nil {
			return err
		}
		defer file.Close()

		return c.Stream(http.StatusOK, "text/html", file)
	})
	return nil
}

// setupDirectoryFallback configures the fallback route for directory-based serving
func setupDirectoryFallback(echoRouter *echo.Echo, handler http.Handler) error {
	echoRouter.GET("/*", func(c echo.Context) error {
		if !strings.HasPrefix(c.Request().URL.Path, "/api/") {
			c.Request().URL.Path = "/"
			handler.ServeHTTP(c.Response(), c.Request())
		}
		return nil
	})
	return nil
}

// MustSetupStaticRoutes is a panic-on-error version of SetupStaticRoutes
func MustSetupStaticRoutes(r Router, cfg StaticConfig) {
	if err := SetupStaticRoutes(r, cfg); err != nil {
		panic(fmt.Sprintf("Failed to setup static routes: %v", err))
	}
}
