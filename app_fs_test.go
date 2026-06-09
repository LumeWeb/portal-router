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

	"golang.org/x/net/html"
)

func TestNewAppFilesystem(t *testing.T) {
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`,
		"static/app.js":  `console.log("app.js");`,
	})

	portalDomain := "example.com"
	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{Domain: portalDomain})

	if appFS == nil {
		t.Fatal("NewAppFilesystem returned nil")
	}

	if appFS.config.Domain != portalDomain {
		t.Errorf("Domain not set correctly. Expected: %s, got: %s", portalDomain, appFS.config.Domain)
	}

	if appFS.isSubdomain {
		t.Error("isSubdomain should be false for example.com")
	}

	appFS = NewAppFilesystem(fsys, AppFilesystemConfig{Domain: "sub.example.co.uk"})

	if !appFS.isSubdomain {
		t.Error("isSubdomain should be true for sub.example.co.uk")
	}
}

func TestAppFilesystem_Open_NonIndex(t *testing.T) {
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`,
		"static/app.js":  `console.log("app.js");`,
	})

	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{Domain: "example.com"})

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

	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{Domain: "example.com"})

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

	// Domain and isRoot should be in separate script tags
	if !strings.Contains(contentStr, "window.VITE_PORTAL_DOMAIN = 'example.com'") {
		t.Errorf("Modified index.html does not contain portal domain: %s", contentStr)
	}

	if !strings.Contains(contentStr, "window.VITE_PORTAL_DOMAIN_IS_ROOT = false") {
		t.Errorf("Modified index.html does not contain VITE_PORTAL_DOMAIN_IS_ROOT = false: %s", contentStr)
	}

	if strings.Contains(contentStr, "VITE_PORTAL_BRAND") {
		t.Errorf("Modified index.html should not contain VITE_PORTAL_BRAND when not configured: %s", contentStr)
	}

	// Verify both script tags exist
	scriptCount := strings.Count(contentStr, `<script type="text/javascript">`)
	if scriptCount != 2 {
		t.Errorf("Expected 2 script tags, got %d: %s", scriptCount, contentStr)
	}

	// Parse HTML to verify script position
	doc, err := html.Parse(strings.NewReader(contentStr))
	if err != nil {
		t.Fatalf("Failed to parse modified HTML: %v", err)
	}

	head := findHeadElement(doc)
	if head == nil {
		t.Fatal("No head element found in modified HTML")
	}

	// Verify first child is a script tag
	if head.FirstChild == nil || head.FirstChild.Data != "script" {
		t.Error("First child of head is not a script tag")
	}
}

func TestAppFilesystem_Open_Index_EmptyHead(t *testing.T) {
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head></head><body><h1>Hello</h1></body></html>`,
	})

	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{Domain: "example.com"})

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
	if !strings.Contains(contentStr, "<script") {
		t.Errorf("Modified index.html does not contain script tag: %s", contentStr)
	}

	doc, err := html.Parse(strings.NewReader(contentStr))
	if err != nil {
		t.Fatalf("Failed to parse modified HTML: %v", err)
	}

	head := findHeadElement(doc)
	if head == nil {
		t.Fatal("No head element found in modified HTML")
	}

	if head.FirstChild == nil || head.FirstChild.Data != "script" {
		t.Error("Script tag is not first child of head")
	}
}

func TestAppFilesystem_Open_Index_Cached(t *testing.T) {
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`,
		"static/app.js":  `console.log("app.js");`,
	})

	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{Domain: "example.com"})

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

	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{Domain: "example.com"})

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

func TestAppFilesystem_BrandInjection(t *testing.T) {
	brandJSON := `{"tagline":"Custom Portal","logoUrl":"https://example.com/logo.svg","social":{"github":"https://github.com/test"}}`

	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head></head><body><h1>Hello</h1></body></html>`,
	})

	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{
		Domain:    "example.com",
		BrandJSON: brandJSON,
	})

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

	if !strings.Contains(contentStr, "window.VITE_PORTAL_BRAND = ") {
		t.Errorf("Modified index.html does not contain VITE_PORTAL_BRAND: %s", contentStr)
	}

	// Brand value should be JSON-stringified (double-quoted string)
	if !strings.Contains(contentStr, `window.VITE_PORTAL_BRAND = "{\"tagline\":\"Custom Portal\"`) {
		t.Errorf("VITE_PORTAL_BRAND not properly JSON-encoded: %s", contentStr)
	}

	// Should have 3 script tags: domain, isRoot, brand
	scriptCount := strings.Count(contentStr, `<script type="text/javascript">`)
	if scriptCount != 3 {
		t.Errorf("Expected 3 script tags, got %d: %s", scriptCount, contentStr)
	}
}

func TestAppFilesystem_BrandLogoReplacement(t *testing.T) {
	brandJSON := `{"logoUrl":"https://branded.com/logo.png"}`

	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head></head><body><div data-loader-logo class="w-28 h-28"><svg>old</svg></div></body></html>`,
	})

	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{
		Domain:    "example.com",
		BrandJSON: brandJSON,
	})

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

	if !strings.Contains(contentStr, `<img alt="Logo" src="https://branded.com/logo.png"`) {
		t.Errorf("Logo not replaced: %s", contentStr)
	}

	if strings.Contains(contentStr, "<svg>old</svg>") {
		t.Errorf("Old logo content not removed: %s", contentStr)
	}

	// data-loader-logo div should still exist
	if !strings.Contains(contentStr, `data-loader-logo`) {
		t.Errorf("data-loader-logo div removed: %s", contentStr)
	}
}

func TestAppFilesystem_BrandWithoutLogoNoReplacement(t *testing.T) {
	brandJSON := `{"tagline":"No Logo Portal"}`

	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head></head><body><div data-loader-logo class="w-28 h-28"><svg>old</svg></div></body></html>`,
	})

	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{
		Domain:    "example.com",
		BrandJSON: brandJSON,
	})

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

	// Original logo content should still be present when no logoUrl
	if !strings.Contains(contentStr, "<svg>old</svg>") {
		t.Errorf("Original logo content should not be replaced when no logoUrl: %s", contentStr)
	}
}

func TestAppFilesystem_BrandOnlyNoDomain(t *testing.T) {
	brandJSON := `{"tagline":"Brand Only"}`

	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head></head><body></body></html>`,
	})

	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{BrandJSON: brandJSON})

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

	if strings.Contains(contentStr, "VITE_PORTAL_DOMAIN") {
		t.Errorf("Should not contain VITE_PORTAL_DOMAIN when domain not set: %s", contentStr)
	}

	if !strings.Contains(contentStr, "VITE_PORTAL_BRAND") {
		t.Errorf("Should contain VITE_PORTAL_BRAND: %s", contentStr)
	}
}

func TestAppFilesystem_InvalidBrandJSON(t *testing.T) {
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head></head><body></body></html>`,
	})

	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{
		Domain:    "example.com",
		BrandJSON: `{invalid json`,
	})

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

	// Domain vars should still work even with invalid brand JSON
	if !strings.Contains(contentStr, "window.VITE_PORTAL_DOMAIN = 'example.com'") {
		t.Errorf("Domain injection should still work with invalid brand JSON: %s", contentStr)
	}

	// Brand should still be injected (as a string, json.Marshal will handle it)
	if !strings.Contains(contentStr, "window.VITE_PORTAL_BRAND = ") {
		t.Errorf("Brand should still be injected even with invalid JSON object: %s", contentStr)
	}

	// No logo replacement should happen when JSON parsing fails
	if appFS.brandLogoURL != "" {
		t.Errorf("brandLogoURL should be empty when brand JSON parsing fails")
	}
}

func findHeadElement(doc *html.Node) *html.Node {
	var head *html.Node
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "head" {
			head = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return head
}

func NewTestFS(files map[string]string) fs.FS {
	mapFS := make(fstest.MapFS)
	for name, content := range files {
		mapFS[name] = &fstest.MapFile{Data: []byte(content), Mode: 0444, ModTime: time.Now()}
	}
	return mapFS
}

func TestAppFilesystemIntegration(t *testing.T) {
	fsys := NewTestFS(map[string]string{
		DefaultIndexFile: `<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`,
		"static/app.js":  `console.log("app.js");`,
	})

	appFS := NewAppFilesystem(fsys, AppFilesystemConfig{Domain: "example.com"})

	handler := http.FileServer(http.FS(appFS))
	server := httptest.NewServer(handler)
	defer server.Close()

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

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "window.VITE_PORTAL_DOMAIN = 'example.com'") {
		t.Errorf("Response body does not contain portal domain: %s", bodyStr)
	}

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

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr = string(body)
	if !strings.Contains(bodyStr, `console.log("app.js");`) {
		t.Errorf("Response body does not contain correct static file content: %s", bodyStr)
	}
}
