package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

var _ fs.FS = (*AppFilesystem)(nil)
var _ fs.FileInfo = (*appFileInfo)(nil)

// loaderLogoRe matches <div data-loader-logo ...>inner content</div>
var loaderLogoRe = regexp.MustCompile(`(<div\s+data-loader-logo\s+[^>]*>)([\s\S]*?)(</div>)`)

type AppFilesystemConfig struct {
	Domain    string
	BrandJSON string
}

type BrandConfig struct {
	LogoURL string `json:"logoUrl"`
}

// AppFilesystem wraps an fs.FS to modify index.html content before serving it.
// It caches the modified content after first read for better performance.
type AppFilesystem struct {
	fs           fs.FS
	cache        *bytes.Buffer
	cached       bool
	mu           sync.Mutex
	config       AppFilesystemConfig
	isSubdomain  bool
	brandLogoURL string
}

// NewAppFilesystem creates a new AppFilesystem wrapping the provided fs.FS.
// It automatically determines if the config.Domain is a subdomain using IsSubdomain.
func NewAppFilesystem(fsys fs.FS, config AppFilesystemConfig) *AppFilesystem {
	a := &AppFilesystem{
		fs:          fsys,
		cache:       new(bytes.Buffer),
		cached:      false,
		config:      config,
		isSubdomain: IsSubdomain(config.Domain),
	}

	if config.BrandJSON != "" {
		var brand BrandConfig
		if err := json.Unmarshal([]byte(config.BrandJSON), &brand); err == nil {
			a.brandLogoURL = brand.LogoURL
		}
	}

	return a
}

// Open implements fs.FS interface.
func (a *AppFilesystem) Open(name string) (fs.File, error) {
	if !strings.HasSuffix(name, DefaultIndexFile) {
		return a.fs.Open(name)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cached {
		return &appCachedFile{
			Reader: bytes.NewReader(a.cache.Bytes()),
			name:   name,
		}, nil
	}

	file, err := a.fs.Open(name)
	if err != nil {
		return nil, err
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	modified := a.modifyIndexHTML(content)

	a.cache.Reset()
	a.cache.Write(modified)
	a.cached = true

	return &appCachedFile{
		Reader: bytes.NewReader(modified),
		name:   name,
	}, nil
}

func (a *AppFilesystem) modifyIndexHTML(content []byte) []byte {
	// Logo replacement on raw HTML before DOM parsing — regex is more reliable
	// than DOM round-tripping for this specific nested-content pattern.
	if a.brandLogoURL != "" {
		replacement := fmt.Sprintf(`$1<img alt="Logo" src="%s" />$3`, html.EscapeString(a.brandLogoURL))
		content = loaderLogoRe.ReplaceAll(content, []byte(replacement))
	}

	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return content // fallback to original if parsing fails
	}

	var head *html.Node
	var findHead func(*html.Node)
	findHead = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "head" {
			head = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findHead(c)
		}
	}
	findHead(doc)

	if head == nil {
		return content // no head tag found
	}

	// Build script tags in reverse order — we prepend each to <head>,
	// so prepending in reverse produces correct final DOM order.
	scripts := a.buildScriptContent()

	for i := len(scripts) - 1; i >= 0; i-- {
		scriptNode := &html.Node{
			Type: html.ElementNode,
			Data: "script",
			Attr: []html.Attribute{
				{Key: "type", Val: "text/javascript"},
			},
		}
		textNode := &html.Node{
			Type: html.TextNode,
			Data: scripts[i],
		}
		scriptNode.AppendChild(textNode)

		if head.FirstChild != nil {
			head.InsertBefore(scriptNode, head.FirstChild)
		} else {
			head.AppendChild(scriptNode)
		}
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return content // fallback to original if rendering fails
	}

	return buf.Bytes()
}

func (a *AppFilesystem) buildScriptContent() []string {
	var scripts []string

	if a.config.Domain != "" {
		scripts = append(scripts, fmt.Sprintf("window.VITE_PORTAL_DOMAIN = '%s';", html.EscapeString(a.config.Domain)))
		scripts = append(scripts, fmt.Sprintf("window.VITE_PORTAL_DOMAIN_IS_ROOT = %t;", a.isSubdomain))
	}

	if a.config.BrandJSON != "" {
		// json.Marshal on a string is equivalent to JS JSON.stringify() —
		// produces a quoted, escaped string literal safe for JS assignment.
		brandJS, err := json.Marshal(a.config.BrandJSON)
		if err != nil {
			brandJS = []byte(`""`)
		}
		scripts = append(scripts, fmt.Sprintf("window.VITE_PORTAL_BRAND = %s;", string(brandJS)))
	}

	return scripts
}

// appCachedFile implements fs.File for cached content
type appCachedFile struct {
	*bytes.Reader
	name string
}

func (f *appCachedFile) Close() error {
	return nil
}

func (f *appCachedFile) Stat() (fs.FileInfo, error) {
	return &appFileInfo{
		name: f.name,
		size: int64(f.Len()),
	}, nil
}

// appFileInfo implements fs.FileInfo for cached files
type appFileInfo struct {
	name string
	size int64
}

func (fi *appFileInfo) Name() string       { return fi.name }
func (fi *appFileInfo) Size() int64        { return fi.size }
func (fi *appFileInfo) Mode() fs.FileMode  { return 0444 } // Read-only
func (fi *appFileInfo) ModTime() time.Time { return time.Now() }
func (fi *appFileInfo) IsDir() bool        { return false }
func (fi *appFileInfo) Sys() any          { return nil }
