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

// WebAppConfig returns a StaticConfig optimized for web applications with:
// - Embedded filesystem
// - SPA fallback to index.html
func WebAppConfig(fsys fs.FS) StaticConfig {
	return StaticConfig{
		FS:        fsys,
		IndexFile: "index.html",
	}
}

// WebAppEnvConfig returns a StaticConfig that reads the filesystem path from an environment variable
func WebAppEnvConfig(envVar string) StaticConfig {
	path := os.Getenv(envVar)
	if path == "" {
		return StaticConfig{}
	}
	return StaticConfig{
		FS:        os.DirFS(path),
		IndexFile: "index.html",
	}
}

// StaticSiteConfig returns a StaticConfig optimized for static sites with:
// - Files from filesystem
// - SPA fallback to index.html
func StaticSiteConfig(fsys fs.FS) StaticConfig {
	return StaticConfig{
		FS:        fsys,
		IndexFile: "index.html",
	}
}

// StaticSiteEnvConfig returns a StaticConfig that reads the filesystem path from an environment variable
func StaticSiteEnvConfig(envVar string) StaticConfig {
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

// DefaultWebAppSetup is a one-liner to setup a standard web app configuration
func DefaultWebAppSetup(r Router, fsys fs.FS) error {
	return SetupStaticRoutes(r, WebAppConfig(fsys))
}

// DefaultWebAppEnvSetup is a one-liner to setup web app from env var
func DefaultWebAppEnvSetup(r Router, envVar string) error {
	return SetupStaticRoutes(r, WebAppEnvConfig(envVar))
}

// MustDefaultWebAppSetup is a panic-on-error version of DefaultWebAppSetup
func MustDefaultWebAppSetup(r Router, fsys fs.FS) {
	MustSetupStaticRoutes(r, WebAppConfig(fsys))
}

// MustDefaultWebAppEnvSetup is a panic-on-error version of DefaultWebAppEnvSetup
func MustDefaultWebAppEnvSetup(r Router, envVar string) {
	MustSetupStaticRoutes(r, WebAppEnvConfig(envVar))
}

// DefaultStaticSiteSetup is a one-liner to setup a standard static site
func DefaultStaticSiteSetup(r Router, fsys fs.FS) error {
	return SetupStaticRoutes(r, StaticSiteConfig(fsys))
}

// DefaultStaticSiteEnvSetup is a one-liner to setup static site from env var
func DefaultStaticSiteEnvSetup(r Router, envVar string) error {
	return SetupStaticRoutes(r, StaticSiteEnvConfig(envVar))
}

// MustDefaultStaticSiteSetup is a panic-on-error version of DefaultStaticSiteSetup
func MustDefaultStaticSiteSetup(r Router, fsys fs.FS) {
	MustSetupStaticRoutes(r, StaticSiteConfig(fsys))
}

// MustDefaultStaticSiteEnvSetup is a panic-on-error version of DefaultStaticSiteEnvSetup
func MustDefaultStaticSiteEnvSetup(r Router, envVar string) {
	MustSetupStaticRoutes(r, StaticSiteEnvConfig(envVar))
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
