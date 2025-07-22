package router

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestNewAppFilesystem(t *testing.T) {
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`,
		"static/app.js":  `console.log("app.js");`,
	})

	portalDomain := "example.com"
	appFS := NewAppFilesystem(fsys, portalDomain)

	if appFS == nil {
		t.Fatal("NewAppFilesystem returned nil")
	}

	if appFS.portalDomain != portalDomain {
		t.Errorf("portalDomain is not set correctly. Expected: %s, got: %s", portalDomain, appFS.portalDomain)
	}

	if appFS.portalIsSubdomain {
		t.Error("portalIsSubdomain should be false for example.com")
	}

	portalDomain = "sub.example.co.uk"
	appFS = NewAppFilesystem(fsys, portalDomain)

	if !appFS.portalIsSubdomain {
		t.Error("portalIsSubdomain should be true for sub.example.co.uk")
	}
}

func TestAppFilesystem_Open_NonIndex(t *testing.T) {
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`,
		"static/app.js":  `console.log("app.js");`,
	})

	portalDomain := "example.com"
	appFS := NewAppFilesystem(fsys, portalDomain)

	file, err := appFS.Open("static/app.js")
	if err != nil {
		t.Fatalf("Failed to open static/app.js: %v", err)
	}
	defer func(file fs.File) {
		err = file.Close()
		if err != nil {
			t.Error(err)
		}
	}(file)

	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("Failed to read static/app.js: %v", err)
	}

	expected := `console.log("app.js");`
	if string(content) != expected {
		t.Errorf("Content of static/app.js is incorrect. Expected: %s, got: %s", expected, string(content))
	}
}

func TestAppFilesystem_Open_Index(t *testing.T) {
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`,
		"static/app.js":  `console.log("app.js");`,
	})

	portalDomain := "example.com"
	appFS := NewAppFilesystem(fsys, portalDomain)

	file, err := appFS.Open(DefaultIndexFile)
	if err != nil {
		t.Fatalf("Failed to open index.html: %v", err)
	}
	defer func(file fs.File) {
		err = file.Close()
		if err != nil {
			t.Error(err)
		}
	}(file)

	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "window.VITE_PORTAL_DOMAIN = 'example.com'") {
		t.Errorf("Modified index.html does not contain portal domain: %s", contentStr)
	}

	if strings.Contains(contentStr, "window.VITE_PORTAL_DOMAIN_IS_ROOT = true") {
		t.Errorf("Modified index.html incorrectly contains portal is root: %s", contentStr)
	}

	if !strings.Contains(contentStr, "<script type=\"text/javascript\">window.VITE_PORTAL_DOMAIN") {
		t.Errorf("Modified index.html does not contain script tag: %s", contentStr)
	}
}

func TestAppFilesystem_Open_Index_Cached(t *testing.T) {
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`,
		"static/app.js":  `console.log("app.js");`,
	})

	portalDomain := "example.com"
	appFS := NewAppFilesystem(fsys, portalDomain)

	// First open to cache the content
	file1, err := appFS.Open(DefaultIndexFile)
	if err != nil {
		t.Fatalf("Failed to open index.html: %v", err)
	}
	defer func(file1 fs.File) {
		err = file1.Close()
		if err != nil {
			t.Error(err)
		}
	}(file1)

	_, err = io.ReadAll(file1)
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}

	// Second open to retrieve from cache
	file2, err := appFS.Open(DefaultIndexFile)
	if err != nil {
		t.Fatalf("Failed to open index.html from cache: %v", err)
	}
	defer func(file2 fs.File) {
		err = file2.Close()
		if err != nil {
			t.Error(err)
		}
	}(file2)

	content, err := io.ReadAll(file2)
	if err != nil {
		t.Fatalf("Failed to read index.html from cache: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "window.VITE_PORTAL_DOMAIN = 'example.com'") {
		t.Errorf("Cached index.html does not contain portal domain: %s", contentStr)
	}
}

func TestAppFilesystem_ModifyIndexHTML_NoHead(t *testing.T) {
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><body><h1>Hello</h1></body></html>`,
	})

	portalDomain := "example.com"
	appFS := NewAppFilesystem(fsys, portalDomain)

	file, err := appFS.Open(DefaultIndexFile)
	if err != nil {
		t.Fatalf("Failed to open index.html: %v", err)
	}
	defer func(file fs.File) {
		err = file.Close()
		if err != nil {
			t.Error(err)
		}
	}(file)

	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("Failed to read index.html: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "window.VITE_PORTAL_DOMAIN = 'example.com'") {
		t.Errorf("Modified index.html does not contain portal domain: %s", contentStr)
	}
}

// NewTestFS creates an in-memory fs.FS for testing.
func NewTestFS(files map[string]string) fs.FS {
	mapFS := make(fstest.MapFS)
	for name, content := range files {
		mapFS[name] = &fstest.MapFile{Data: []byte(content), Mode: 0444, ModTime: time.Now()}
	}
	return mapFS
}

func TestAppFilesystemIntegration(t *testing.T) {
	// Create a test file system
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`,
		"static/app.js":  `console.log("app.js");`,
	})

	// Create an AppFilesystem instance
	portalDomain := "example.com"
	appFS := NewAppFilesystem(fsys, portalDomain)

	// Create a test HTTP server
	handler := http.FileServer(http.FS(appFS))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a request to the index file
	resp, err := http.Get(server.URL + "/" + DefaultIndexFile)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			t.Error(err)
		}
	}(resp.Body)

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Check if the portal domain is injected correctly
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "window.VITE_PORTAL_DOMAIN = 'example.com'") {
		t.Errorf("Response body does not contain portal domain: %s", bodyStr)
	}

	// Make a request to a static file
	resp, err = http.Get(server.URL + "/static/app.js")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			t.Error(err)
		}
	}(resp.Body)

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Read the response body
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Check if the static file content is correct
	bodyStr = string(body)
	if !strings.Contains(bodyStr, `console.log("app.js");`) {
		t.Errorf("Response body does not contain correct static file content: %s", bodyStr)
	}
}
