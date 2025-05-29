package router

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	// StaticAssetsPath is the URL path prefix for static assets
	StaticAssetsPath = "/assets"
	// StaticAssetsDir is the directory name where static assets are stored
	StaticAssetsDir = "assets"
	// DefaultIndexFile is the default filename to serve for SPA fallback
	DefaultIndexFile = "index.html"
)

// httpToFS implements fs.FS by wrapping an http.FileSystem.
// This allows using http.FileSystem implementations (like http.Dir) 
// where fs.FS is required.
type httpToFS struct {
	fs http.FileSystem
}

// Open opens the named file for reading using the underlying http.FileSystem.
// Implements fs.FS interface.
func (h httpToFS) Open(name string) (fs.File, error) {
	return h.fs.Open(name)
}

// fsAdapter converts between filesystem interfaces.
// Accepts either:
// - fs.FS (returned as-is)
// - http.FileSystem (wrapped in httpToFS)
// - nil (returns nil)
// Panics for any other input type.
func fsAdapter(fsys any) fs.FS {
	if fsys == nil {
		return nil
	}
	switch v := fsys.(type) {
	case fs.FS:
		return v
	case http.FileSystem:
		return &httpToFS{fs: v}
	default:
		panic(fmt.Sprintf("fsAdapter: unsupported filesystem type %T", fsys))
	}
}

// Predefined configurations for common static file setups

// StaticConfigWithFS returns a StaticConfig with the given filesystem and SPA fallback to DefaultIndexFile.
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
		IndexFile: DefaultIndexFile,
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
		IndexFile: DefaultIndexFile,
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

// StaticConfig defines configuration options for serving static files.
// Only one of these fields should be set:
// - DirPath or EnvVar for directory-based serving
// - FS for embedded filesystems
// - DefaultHandler for custom handler fallback
type StaticConfig struct {
	// DirPath is the filesystem path to the directory containing static files.
	// If empty, EnvVar will be checked instead.
	DirPath string

	// EnvVar specifies an environment variable containing the static files directory path.
	// Only used if DirPath is empty.
	EnvVar string

	// DefaultHandler is used as a fallback handler when no directory or FS is provided.
	// Typically an http.FileServer or similar.
	DefaultHandler http.Handler

	// FS specifies an embedded filesystem (like embed.FS) to serve files from.
	// Requires IndexFile to be set for SPA fallback behavior.
	FS fs.FS

	// IndexFile is the name of the fallback file to serve for SPA routes (e.g. "index.html").
	// Required when FS is set.
	IndexFile string
}

// SetupStaticRoutes configures static file serving routes with optional SPA fallback.
//
// Supports multiple configuration methods:
// - Directory-based serving via DirPath/EnvVar
// - Embedded filesystems via FS
// - Custom handler fallback via DefaultHandler
//
// When using FS, IndexFile must be specified for SPA fallback behavior.
// Directory-based serving will automatically serve the index file for root requests.
//
// Static assets are served under StaticAssetsPath ("/assets") by default.
// All other requests fall back to the index file for SPA behavior.
//
// Returns an error if:
// - FS is set but IndexFile is empty
// - No valid configuration is provided
func SetupStaticRoutes(r Router, cfg StaticConfig) error {
	// Validate configuration
	if cfg.FS != nil && cfg.IndexFile == "" {
		cfg.IndexFile = DefaultIndexFile
	}

	echoRouter := GetRouter(r)

	// Handle filesystem case (embedded or directory)
	var handler http.Handler
	var fsys fs.FS

	if cfg.FS != nil {
		fsys = cfg.FS
		// Try to get subdirectory for assets if it exists
		var staticFS = fsys
		if _, err := fs.Stat(fsys, StaticAssetsDir); err == nil {
			subFs, err := fs.Sub(fsys, StaticAssetsDir)
			if err == nil {
				staticFS = subFs
			}
		}
		echoRouter.StaticFS(StaticAssetsPath, fsAdapter(staticFS))
	} else {
		dirPath := cfg.DirPath
		if dirPath == "" && cfg.EnvVar != "" {
			dirPath = os.Getenv(cfg.EnvVar)
		}

		if dirPath != "" {
			assetsPath := filepath.Join(dirPath, StaticAssetsDir)
			fsys = os.DirFS(dirPath)
			handler = http.FileServer(http.Dir(dirPath))
			echoRouter.Static(StaticAssetsPath, assetsPath)
		} else if cfg.DefaultHandler != nil {
			handler = cfg.DefaultHandler
		}
	}

	if fsys != nil {
		return setupSPAFallback(echoRouter, fsys, cfg.IndexFile)
	}
	if handler != nil {
		return setupDirectoryFallback(echoRouter, handler)
	}
	return errors.New("must provide either DirPath, DefaultHandler, FS or EnvVar")
}

// setupSPAFallback configures route handlers to serve the index file for all unmatched paths,
// enabling single-page application behavior.
//
// All requests not under "/api/" will return the specified index file.
// API routes are excluded to allow proper 404 responses for invalid API calls.
func setupSPAFallback(echoRouter *echo.Echo, fsys fs.FS, indexFile string) error {
	if indexFile == "" {
		indexFile = DefaultIndexFile
	}
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

// setupDirectoryFallback configures route handlers to serve files from a directory
// using the provided http.Handler.
//
// All requests not under "/api/" will be handled by the provided handler.
// API routes are excluded to allow proper 404 responses for invalid API calls.
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

// MustSetupStaticRoutes wraps SetupStaticRoutes and panics if configuration fails.
// Intended for use during application initialization where configuration errors
// should fail fast.
func MustSetupStaticRoutes(r Router, cfg StaticConfig) {
	if err := SetupStaticRoutes(r, cfg); err != nil {
		panic(fmt.Sprintf("Failed to setup static routes: %v", err))
	}
}
