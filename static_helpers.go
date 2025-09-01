package router

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

// cacheControlWriter wraps http.ResponseWriter to only set Cache-Control
// header when a 200 OK response is written
type cacheControlWriter struct {
	http.ResponseWriter
	cacheControl  string
	headerWritten bool
}

func (w *cacheControlWriter) WriteHeader(code int) {
	if code == http.StatusOK {
		w.Header().Set("Cache-Control", w.cacheControl)
	}
	w.ResponseWriter.WriteHeader(code)
	w.headerWritten = true
}

func (w *cacheControlWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

const (
	// StaticAssetsPath is the URL path prefix for static assets
	StaticAssetsPath = "/static"
	// DefaultIndexFile is the default filename to serve for SPA fallback
	DefaultIndexFile = "index.html"
)

var (
	// StaticAssetsDir is the directory name where static assets are stored (derived from StaticAssetsPath)
	StaticAssetsDir = strings.TrimPrefix(StaticAssetsPath, "/")
	// FaviconFiles lists the supported favicon file extensions
	FaviconFiles = "ico,png,svg,gif"
)

var faviconExts = strings.Split(FaviconFiles, ",")

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

// getAssetsSubFS tries to get a sub-filesystem for the assets directory if it exists.
// Falls back to the original filesystem if the assets directory doesn't exist.
func getAssetsSubFS(fsys fs.FS, assetsDir string) fs.FS {
	staticFS := fsys
	if _, err := fs.Stat(fsys, assetsDir); err == nil {
		if subFs, err := fs.Sub(fsys, assetsDir); err == nil {
			staticFS = subFs
		}
	}
	return staticFS
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
		AssetsDir: StaticAssetsDir,
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
		AssetsDir: StaticAssetsDir,
	}
}

// PublicFilesConfig returns a StaticConfig for serving public files from disk
func PublicFilesConfig(dirPath string) StaticConfig {
	return StaticConfig{
		DirPath:   dirPath,
		AssetsDir: StaticAssetsDir,
	}
}

// PublicFilesEnvConfig returns a StaticConfig that reads the directory path from an environment variable
func PublicFilesEnvConfig(envVar string) StaticConfig {
	path := os.Getenv(envVar)
	return StaticConfig{
		DirPath:   path,
		EnvVar:    envVar,
		AssetsDir: StaticAssetsDir,
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

// MPASetup is a shorthand helper to configure Multi-Page Application support
// with the given filesystem. The fsys parameter can be either:
// - fs.FS (for embedded files)
// - http.FileSystem (for directory-based files)
func MPASetup(r Router, fsys any) error {
	return SetupStaticRoutes(r, StaticConfig{
		FS:  fsAdapter(fsys),
		MPA: true,
	})
}

// MPASetupWithAssets is a shorthand helper to configure Multi-Page Application support
// with the given filesystem and custom assets directory. The fsys parameter can be either:
// - fs.FS (for embedded files)
// - http.FileSystem (for directory-based files)
func MPASetupWithAssets(r Router, fsys any, assetsDir string) error {
	return SetupStaticRoutes(r, StaticConfig{
		FS:        fsAdapter(fsys),
		MPA:       true,
		AssetsDir: assetsDir,
	})
}

// MustMPASetup is a panic-on-error version of MPASetup
func MustMPASetup(r Router, fsys any) {
	if err := MPASetup(r, fsys); err != nil {
		panic(fmt.Sprintf("Failed to setup MPA routes: %v", err))
	}
}

// MustMPASetupWithAssets is a panic-on-error version of MPASetupWithAssets
func MustMPASetupWithAssets(r Router, fsys any, assetsDir string) {
	if err := MPASetupWithAssets(r, fsys, assetsDir); err != nil {
		panic(fmt.Sprintf("Failed to setup MPA routes with assets dir %s: %v", assetsDir, err))
	}
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

	// AssetsDir specifies the directory name where static assets are stored.
	// Defaults to StaticAssetsDir ("static") if empty.
	AssetsDir string

	// DefaultHandler is used as a fallback handler when no directory or FS is provided.
	// Typically an http.FileServer or similar.
	DefaultHandler http.Handler

	// FS specifies an embedded filesystem (like embed.FS) to serve files from.
	FS fs.FS

	// IndexFile is the name of the fallback file to serve for SPA routes (e.g. "index.html").
	// Only used when MPA is false.
	IndexFile string

	// MPA enables Multi-Page Application mode where files are served directly instead of SPA fallback.
	// When true, IndexFile is ignored.
	MPA bool
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
// Static assets are served under StaticAssetsPath ("/static") by default.
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

	// Resolve assets directory name and URL mount prefix
	assetsDir := cfg.AssetsDir
	if assetsDir == "" {
		assetsDir = StaticAssetsDir
	}
	assetsPrefix := assetsDir
	if !strings.HasPrefix(assetsPrefix, "/") {
		assetsPrefix = "/" + assetsPrefix
	}

	// Setup filesystem based on configuration
	if cfg.FS != nil {
		fsys = cfg.FS
		staticFS := getAssetsSubFS(fsys, assetsDir)
		echoRouter.StaticFS(assetsPrefix, fsAdapter(staticFS))
	} else {
		// Handle directory-based serving
		dirPath := cfg.DirPath
		if dirPath == "" && cfg.EnvVar != "" {
			dirPath = os.Getenv(cfg.EnvVar)
		}

		if dirPath != "" {
			fsys = os.DirFS(dirPath)
			staticFS := getAssetsSubFS(fsys, assetsDir)
			echoRouter.StaticFS(assetsPrefix, fsAdapter(staticFS))
		} else if cfg.DefaultHandler != nil {
			handler = cfg.DefaultHandler
		}
	}

	// Setup favicon routes first
	setupFaviconRoutes(echoRouter, fsys, handler, cfg)

	// Register routes based on configuration
	switch {
	case fsys != nil && cfg.MPA:
		// MPA mode - serve files directly
		echoRouter.StaticFS("/", fsAdapter(fsys))
	case fsys != nil:
		// SPA mode - fallback to index file
		return setupSPAFallback(echoRouter, fsys, cfg.IndexFile)
	case cfg.DefaultHandler != nil:
		// Use default handler fallback
		echoRouter.GET("/*", func(c echo.Context) error {
			if strings.HasPrefix(c.Request().URL.Path, "/api/") {
				return echo.ErrNotFound
			}
			handler.ServeHTTP(c.Response(), c.Request())
			return nil
		})
	default:
		return errors.New("must provide either DirPath, DefaultHandler, FS or EnvVar")
	}
	return nil
}

// setupSPAFallback configures route handlers to serve the index file for all unmatched paths,
// enabling single-page application behavior.
//
// All requests not under "/api/" will return the specified index file.
// API routes are excluded to allow proper 404 responses for invalid API calls.
// Favicon files are served from the root (favicon.*) or StaticAssetsDir if present.
func setupSPAFallback(echoRouter *echo.Echo, fsys fs.FS, indexFile string) error {
	if indexFile == "" {
		indexFile = DefaultIndexFile
	}

	echoRouter.GET("/*", func(c echo.Context) error {
		if strings.HasPrefix(c.Request().URL.Path, "/api/") {
			return echo.ErrNotFound
		}
		return fsFile(c, indexFile, fsys)
	})
	return nil
}

func createFaviconHandler(fsys fs.FS, handler http.Handler, cfg StaticConfig, ext string) echo.HandlerFunc {
	if fsys != nil {
		return func(c echo.Context) error {
			faviconPath := strings.TrimPrefix(c.Path(), "/")
			file, err := fsys.Open(faviconPath)
			if err != nil {
				// Try static directory
				assetsDir := cfg.AssetsDir
				if assetsDir == "" {
					assetsDir = StaticAssetsDir
				}
				file, err = fsys.Open(path.Join(assetsDir, faviconPath))
				if err != nil {
					return echo.ErrNotFound
				}
			}
			defer file.Close()

			var contentType string
			switch ext {
			case "png":
				contentType = "image/png"
			case "svg":
				contentType = "image/svg+xml"
			case "gif":
				contentType = "image/gif"
			default:
				contentType = "image/x-icon"
			}

			h := c.Response().Header()
			h.Set("Content-Type", contentType)
			h.Set("Cache-Control", "public, max-age=31536000, immutable")

			if c.Request().Method == http.MethodHead {
				return c.NoContent(http.StatusOK)
			}
			return c.Stream(http.StatusOK, contentType, file)
		}
	} else if handler != nil {
		return func(c echo.Context) error {
			c.Request().URL.Path = "/favicon." + ext
			handler.ServeHTTP(c.Response(), c.Request())
			return nil
		}
	}
	return nil
}

func setupFaviconRoutes(echoRouter *echo.Echo, fsys fs.FS, handler http.Handler, cfg StaticConfig) {
	for _, ext := range faviconExts {
		ext := ext // capture range variable
		serve := createFaviconHandler(fsys, handler, cfg, ext)
		if serve != nil {
			echoRouter.GET("/favicon."+ext, serve)
			echoRouter.HEAD("/favicon."+ext, serve)
		}
	}
}

// setupDirectoryFallback configures route handlers to serve files from a directory
// using the provided http.Handler.
//
// All requests not under "/api/" will be handled by the provided handler.
// API routes are excluded to allow proper 404 responses for invalid API calls.
// Favicon files are served from the root if they exist in the filesystem.
func setupDirectoryFallback(echoRouter *echo.Echo, handler http.Handler) error {
	// Add favicon route handlers
	for _, ext := range faviconExts {
		ext := ext // capture range variable
		serve := func(c echo.Context) error {
			// Wrap response writer to only set cache headers on 200 OK
			origWriter := c.Response().Writer
			cacheWriter := &cacheControlWriter{
				ResponseWriter: origWriter,
				cacheControl:   "public, max-age=31536000, immutable",
			}
			c.Response().Writer = cacheWriter
			defer func() {
				c.Response().Writer = origWriter
			}()

			c.Request().URL.Path = "/favicon." + ext
			handler.ServeHTTP(c.Response(), c.Request())
			return nil
		}
		echoRouter.GET("/favicon."+ext, serve)
		echoRouter.HEAD("/favicon."+ext, serve)
	}

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

func fsFile(c echo.Context, file string, filesystem fs.FS) error {
	f, err := filesystem.Open(file)
	if err != nil {
		return echo.ErrNotFound
	}
	defer f.Close()

	fi, _ := f.Stat()
	if fi.IsDir() {
		file = filepath.ToSlash(filepath.Join(file, DefaultIndexFile)) // ToSlash is necessary for Windows. fs.Open and os.Open are different in that aspect.
		f, err = filesystem.Open(file)
		if err != nil {
			return echo.ErrNotFound
		}
		defer f.Close()
		if fi, err = f.Stat(); err != nil {
			return err
		}
	}
	ff, ok := f.(io.ReadSeeker)
	if !ok {
		return errors.New("file does not implement io.ReadSeeker")
	}
	http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), ff)
	return nil
}
