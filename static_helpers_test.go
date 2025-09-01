package router

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestStaticFileServing(t *testing.T) {
	t.Run("serves favicon files from root", func(t *testing.T) {
		// Create temp dir with favicon files
		dir := t.TempDir()
		
		cases := []struct {
			path   string
			body   string
			ctype  string
		}{
			{"/favicon.ico", "ico content", "image/x-icon"},
			{"/favicon.png", "png content", "image/png"},
			{"/favicon.svg", "svg content", "image/svg+xml"},
			{"/favicon.gif", "gif content", "image/gif"},
		}

		// Create favicon files in root
		for _, tc := range cases {
			err := os.WriteFile(filepath.Join(dir, tc.path[1:]), []byte(tc.body), 0644)
			require.NoError(t, err)
		}

		// Setup router with static config
		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		err = SetupStaticRoutes(r, StaticConfig{DirPath: dir})
		require.NoError(t, err)

		for _, tc := range cases {
			t.Run(tc.path, func(t *testing.T) {
				// Test GET
				req := httptest.NewRequest("GET", tc.path, nil)
				rr := httptest.NewRecorder()
				GetRouter(r).ServeHTTP(rr, req)
				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, tc.body, rr.Body.String())
				assert.Equal(t, tc.ctype, rr.Header().Get("Content-Type"))
				assert.Equal(t, "public, max-age=31536000, immutable", rr.Header().Get("Cache-Control"))

				// Test HEAD
				req = httptest.NewRequest("HEAD", tc.path, nil)
				rr = httptest.NewRecorder()
				GetRouter(r).ServeHTTP(rr, req)
				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, "", rr.Body.String())
				assert.Equal(t, tc.ctype, rr.Header().Get("Content-Type"))
				assert.Equal(t, "public, max-age=31536000, immutable", rr.Header().Get("Cache-Control"))
			})
		}
	})

	t.Run("returns 404 for missing favicon files", func(t *testing.T) {
		// Create temp dir without favicon files
		dir := t.TempDir()

		// Setup router with static config
		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		err = SetupStaticRoutes(r, StaticConfig{DirPath: dir})
		require.NoError(t, err)

		// Test missing favicon returns 404
		req := httptest.NewRequest("GET", "/favicon.ico", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
	t.Run("serves files from directory", func(t *testing.T) {
		// Create temp dir with test file under assets subdir
		dir := t.TempDir()
		assetsDir := filepath.Join(dir, StaticAssetsDir)
		err := os.Mkdir(assetsDir, 0755)
		require.NoError(t, err)

		testFile := filepath.Join(assetsDir, "test.js")
		err = os.WriteFile(testFile, []byte("test content"), 0644)
		require.NoError(t, err)

		// Setup router with static config
		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		err = SetupStaticRoutes(r, StaticConfig{DirPath: dir})
		require.NoError(t, err)

		// Test file serving
		req := httptest.NewRequest("GET", StaticAssetsPath+"/test.js", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "test content", rr.Body.String())
	})

	t.Run("serves files with special characters", func(t *testing.T) {
		// Create temp dir with test file containing special chars under assets subdir
		dir := t.TempDir()
		assetsDir := filepath.Join(dir, StaticAssetsDir)
		err := os.Mkdir(assetsDir, 0755)
		require.NoError(t, err)

		testFile := filepath.Join(assetsDir, "test__special__file.js")
		err = os.WriteFile(testFile, []byte("special content"), 0644)
		require.NoError(t, err)

		// Setup router with static config
		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		err = SetupStaticRoutes(r, StaticConfig{DirPath: dir})
		require.NoError(t, err)

		// Test file serving
		req := httptest.NewRequest("GET", StaticAssetsPath+"/test__special__file.js", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "special content", rr.Body.String())
	})

	t.Run("uses default handler when no dir path", func(t *testing.T) {
		// Setup router with default handler
		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		called := false
		defaultHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			_, err := w.Write([]byte("default handler"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		})

		err = SetupStaticRoutes(r, StaticConfig{
			DefaultHandler: defaultHandler,
		})
		require.NoError(t, err)

		// Test handler is called
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)

		assert.True(t, called)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "default handler", rr.Body.String())
	})
}

func TestFSAdapter(t *testing.T) {
	t.Run("adapts fs.FS", func(t *testing.T) {
		mock := fstest.MapFS{"test.txt": {Data: []byte("content")}}
		result := fsAdapter(mock)
		assert.Equal(t, mock, result)
	})

	t.Run("adapts http.FileSystem", func(t *testing.T) {
		mock := http.Dir(".")
		result := fsAdapter(mock)
		_, ok := result.(*httpToFS)
		assert.True(t, ok)
	})

	t.Run("adapts nil", func(t *testing.T) {
		result := fsAdapter(nil)
		assert.Nil(t, result)
	})

	t.Run("panics on invalid type", func(t *testing.T) {
		assert.Panics(t, func() {
			fsAdapter("invalid")
		})
	})
}

func TestStaticFileServingFS(t *testing.T) {
	t.Run("serves favicon files from fs.FS", func(t *testing.T) {
		cases := []struct {
			path   string
			body   string
			ctype  string
		}{
			{"/favicon.ico", "ico content", "image/x-icon"},
			{"/favicon.png", "png content", "image/png"},
			{"/favicon.svg", "svg content", "image/svg+xml"},
			{"/favicon.gif", "gif content", "image/gif"},
		}

		mockFS := fstest.MapFS{
			"index.html": &fstest.MapFile{
				Data: []byte("<html>SPA</html>"),
				Mode: 0644,
			},
		}
		for _, tc := range cases {
			mockFS[tc.path[1:]] = &fstest.MapFile{
				Data: []byte(tc.body),
				Mode: 0644,
			}
		}

		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		err = SetupStaticRoutes(r, StaticConfig{
			FS:        mockFS,
			IndexFile: "index.html",
		})
		require.NoError(t, err)

		for _, tc := range cases {
			t.Run(tc.path, func(t *testing.T) {
				// Test GET
				req := httptest.NewRequest("GET", tc.path, nil)
				rr := httptest.NewRecorder()
				GetRouter(r).ServeHTTP(rr, req)
				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, tc.body, rr.Body.String())
				assert.Equal(t, tc.ctype, rr.Header().Get("Content-Type"))
				assert.Equal(t, "public, max-age=31536000, immutable", rr.Header().Get("Cache-Control"))

				// Test HEAD
				req = httptest.NewRequest("HEAD", tc.path, nil)
				rr = httptest.NewRecorder()
				GetRouter(r).ServeHTTP(rr, req)
				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, "", rr.Body.String())
				assert.Equal(t, tc.ctype, rr.Header().Get("Content-Type"))
				assert.Equal(t, "public, max-age=31536000, immutable", rr.Header().Get("Cache-Control"))
			})
		}
	})

	t.Run("serves favicon files from static directory in fs.FS", func(t *testing.T) {
		mockFS := fstest.MapFS{
			StaticAssetsDir + "/favicon.ico": &fstest.MapFile{
				Data: []byte("ico content"),
				Mode: 0644,
			},
			"index.html": &fstest.MapFile{
				Data: []byte("<html>SPA</html>"),
				Mode: 0644,
			},
		}

		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		err = SetupStaticRoutes(r, StaticConfig{
			FS:        mockFS,
			IndexFile: "index.html",
		})
		require.NoError(t, err)

		// Test favicon serving from static directory
		req := httptest.NewRequest("GET", "/favicon.ico", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "ico content", rr.Body.String())
		assert.Equal(t, "image/x-icon", rr.Header().Get("Content-Type"))
	})
	t.Run("serves files from MapFS", func(t *testing.T) {
		mockFS := fstest.MapFS{
			StaticAssetsDir + "/test.txt": &fstest.MapFile{
				Data: []byte("test content"),
				Mode: 0644,
			},
			"index.html": &fstest.MapFile{
				Data: []byte("<html>SPA</html>"),
				Mode: 0644,
			},
		}

		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		err = SetupStaticRoutes(r, StaticConfig{
			FS:        mockFS,
			IndexFile: "index.html",
		})
		require.NoError(t, err)

		req := httptest.NewRequest("GET", StaticAssetsPath+"/test.txt", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "test content", rr.Body.String())

		req = httptest.NewRequest("GET", "/unknown", nil)
		rr = httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "<html>SPA</html>", rr.Body.String())
	})

	t.Run("returns 404 for missing files", func(t *testing.T) {
		mockFS := fstest.MapFS{}

		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		err = SetupStaticRoutes(r, StaticConfig{
			FS:        mockFS,
			IndexFile: "index.html",
		})
		require.NoError(t, err)

		req := httptest.NewRequest("GET", StaticAssetsPath+"/missing.txt", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestConfigHelpers(t *testing.T) {
	t.Run("StaticConfigWithFS", func(t *testing.T) {
		mockFS := fstest.MapFS{}
		cfg := StaticConfigWithFS(mockFS)
		assert.Equal(t, mockFS, cfg.FS)
		assert.Equal(t, "index.html", cfg.IndexFile)
	})

	t.Run("StaticConfigFromEnv", func(t *testing.T) {
		t.Setenv("TEST_STATIC_PATH", "testdata")
		cfg := StaticConfigFromEnv("TEST_STATIC_PATH")
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

	t.Run("DefaultStaticSetup", func(t *testing.T) {
		mockFS := fstest.MapFS{}
		err := DefaultStaticSetup(r, mockFS)
		assert.NoError(t, err)
	})

	t.Run("DefaultStaticEnvSetup", func(t *testing.T) {
		t.Setenv("TEST_STATIC_PATH", "testdata")
		err := DefaultStaticEnvSetup(r, "TEST_STATIC_PATH")
		assert.NoError(t, err)
	})

	t.Run("MustDefaultStaticSetup", func(t *testing.T) {
		mockFS := fstest.MapFS{}
		assert.NotPanics(t, func() {
			MustDefaultStaticSetup(r, mockFS)
		})
	})

	t.Run("MustDefaultStaticEnvSetup", func(t *testing.T) {
		t.Setenv("TEST_STATIC_PATH", "testdata")
		assert.NotPanics(t, func() {
			MustDefaultStaticEnvSetup(r, "TEST_STATIC_PATH")
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

	t.Run("fs with empty index file uses default", func(t *testing.T) {
		mockFS := fstest.MapFS{
			DefaultIndexFile: &fstest.MapFile{
				Data: []byte("<html>default</html>"),
			},
		}
		err := SetupStaticRoutes(r, StaticConfig{
			FS:        mockFS,
			IndexFile: "",
		})
		assert.NoError(t, err)

		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "<html>default</html>", rr.Body.String())
	})

	t.Run("invalid config", func(t *testing.T) {
		err := SetupStaticRoutes(r, StaticConfig{})
		assert.Error(t, err)
	})

	t.Run("with filesystem", func(t *testing.T) {
		mockFS := fstest.MapFS{}
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

func TestMPASetupWithAssets(t *testing.T) {
	mockFS := fstest.MapFS{
		"custom_assets/style.css": &fstest.MapFile{
			Data: []byte("body { color: red; }"),
		},
		"index.html": &fstest.MapFile{
			Data: []byte("<html>index</html>"),
		},
	}

	r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
	require.NoError(t, err)

	err = MPASetupWithAssets(r, mockFS, "custom_assets")
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/custom_assets/style.css", nil)
	rr := httptest.NewRecorder()
	GetRouter(r).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "body { color: red; }", rr.Body.String())
}

func TestMustMPASetupWithAssets(t *testing.T) {
	r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
	require.NoError(t, err)

	mockFS := fstest.MapFS{
		"custom_assets/style.css": &fstest.MapFile{
			Data: []byte("body { color: red; }"),
		},
	}

	assert.NotPanics(t, func() {
		MustMPASetupWithAssets(r, mockFS, "custom_assets")
	})
}

func TestMPASupport(t *testing.T) {
	t.Run("serves multiple html pages from embedded FS", func(t *testing.T) {
		mockFS := fstest.MapFS{
			"index.html": &fstest.MapFile{
				Data: []byte("<html>index</html>"),
			},
			"about.html": &fstest.MapFile{
				Data: []byte("<html>about</html>"), 
			},
			"contact.html": &fstest.MapFile{
				Data: []byte("<html>contact</html>"),
			},
			StaticAssetsDir + "/style.css": &fstest.MapFile{
				Data: []byte("body { color: red; }"),
			},
		}

		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		err = SetupStaticRoutes(r, StaticConfig{
			FS:  mockFS,
			MPA: true, // Enable MPA mode
		})
		require.NoError(t, err)

		cases := []struct {
			path string
			want string
		}{
			{"/", "<html>index</html>"},
			{"/about.html", "<html>about</html>"},
			{"/contact.html", "<html>contact</html>"},
			{StaticAssetsPath + "/style.css", "body { color: red; }"},
		}

		for _, tc := range cases {
			t.Run(tc.path, func(t *testing.T) {
				req := httptest.NewRequest("GET", tc.path, nil)
				rr := httptest.NewRecorder()
				GetRouter(r).ServeHTTP(rr, req)

				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, tc.want, rr.Body.String())
			})
		}

		// Test API routes still return 404
		req := httptest.NewRequest("GET", "/api/test", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("serves multiple html pages from directory", func(t *testing.T) {
		dir := t.TempDir()
		files := map[string]string{
			"index.html":   "<html>index</html>",
			"about.html":   "<html>about</html>",
			"contact.html": "<html>contact</html>",
			StaticAssetsDir + "/style.css": "body { color: red; }",
		}

		for name, content := range files {
			path := filepath.Join(dir, name)
			err := os.MkdirAll(filepath.Dir(path), 0755)
			require.NoError(t, err)
			err = os.WriteFile(path, []byte(content), 0644)
			require.NoError(t, err)
		}

		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		err = SetupStaticRoutes(r, StaticConfig{
			DirPath: dir,
			MPA:     true, // Enable MPA mode
		})
		require.NoError(t, err)

		cases := []struct {
			path string
			want string
		}{
			{"/", "<html>index</html>"},
			{"/about.html", "<html>about</html>"},
			{"/contact.html", "<html>contact</html>"},
			{StaticAssetsPath + "/style.css", "body { color: red; }"},
		}

		for _, tc := range cases {
			t.Run(tc.path, func(t *testing.T) {
				req := httptest.NewRequest("GET", tc.path, nil)
				rr := httptest.NewRecorder()
				GetRouter(r).ServeHTTP(rr, req)

				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, tc.want, rr.Body.String())
			})
		}
	})

	t.Run("ignores indexFile in MPA mode", func(t *testing.T) {
		mockFS := fstest.MapFS{
			"index.html": &fstest.MapFile{
				Data: []byte("<html>index</html>"),
			},
			"other.html": &fstest.MapFile{
				Data: []byte("<html>other</html>"),
			},
		}

		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		err = SetupStaticRoutes(r, StaticConfig{
			FS:        mockFS,
			IndexFile: "other.html", // Should be ignored
			MPA:       true,
		})
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "<html>index</html>", rr.Body.String())
	})
}

func TestSPAFallback(t *testing.T) {
	t.Run("serves index.html for unknown paths", func(t *testing.T) {
		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		// Create temp dir with index.html
		dir := t.TempDir()
		err = os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>SPA</html>"), 0644)
		require.NoError(t, err)
		testFS := os.DirFS(dir)

		err = setupSPAFallback(GetRouter(r), testFS, "index.html")
		require.NoError(t, err)

		// Test SPA fallback
		req := httptest.NewRequest("GET", "/unknown/path", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "<html>SPA</html>", rr.Body.String())
	})

	t.Run("does not interfere with API routes", func(t *testing.T) {
		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)

		// Create temp dir with index.html
		dir := t.TempDir()
		err = os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>SPA</html>"), 0644)
		require.NoError(t, err)
		testFS := os.DirFS(dir)

		err = setupSPAFallback(GetRouter(r), testFS, "index.html")
		require.NoError(t, err)

		// Test API route returns 404
		req := httptest.NewRequest("GET", "/api/test", nil)
		rr := httptest.NewRecorder()
		GetRouter(r).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("serves index.html for root path", func(t *testing.T) {
		r, err := NewRouter(APIInfo().Title("Test").Version("1.0"))
		require.NoError(t, err)
		echoRouter := GetRouter(r)

		// Create temp dir with index.html
		dir := t.TempDir()
		err = os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0644)
		require.NoError(t, err)
		testFS := os.DirFS(dir)

		err = setupSPAFallback(echoRouter, testFS, "index.html")
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		echoRouter.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "<html></html>", rec.Body.String())
	})
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
