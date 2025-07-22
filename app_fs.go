package router

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"
	"text/template"
	"time"

	"golang.org/x/net/html"
)

const scriptTemplate = `window.VITE_PORTAL_DOMAIN = '{{.Domain}}';
window.VITE_PORTAL_DOMAIN_IS_ROOT = {{.IsSubdomain}};`

var _ fs.FS = (*AppFilesystem)(nil)
var _ fs.FileInfo = (*appFileInfo)(nil)

// AppFilesystem wraps an fs.FS to modify index.html content before serving it.
// It caches the modified content after first read for better performance.
type AppFilesystem struct {
	fs                fs.FS
	cache             *bytes.Buffer
	cached            bool
	mu                sync.Mutex
	portalDomain      string
	portalIsSubdomain bool
	scriptTmpl        *template.Template
}

// NewAppFilesystem creates a new AppFilesystem wrapping the provided fs.FS.
// It automatically determines if the portalDomain is a subdomain using IsSubdomain.
func NewAppFilesystem(fsys fs.FS, portalDomain string) *AppFilesystem {
	tmpl, err := template.New("portalVars").Parse(scriptTemplate)
	if err != nil {
		panic(fmt.Sprintf("failed to parse portal vars template: %v", err))
	}

	return &AppFilesystem{
		fs:                fsys,
		cache:             new(bytes.Buffer),
		cached:            false,
		portalDomain:      portalDomain,
		portalIsSubdomain: IsSubdomain(portalDomain),
		scriptTmpl:        tmpl,
	}
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

	// Modify the content as needed
	modified := a.modifyIndexHTML(content)

	// Cache the modified content
	a.cache.Reset()
	a.cache.Write(modified)
	a.cached = true

	return &appCachedFile{
		Reader: bytes.NewReader(modified),
		name:   name,
	}, nil
}

// modifyIndexHTML processes the index.html content to inject portal variables
func (a *AppFilesystem) modifyIndexHTML(content []byte) []byte {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return content // fallback to original if parsing fails
	}

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

	if head == nil {
		return content // no head tag found
	}

	// Create script node with portal variables
	script := &html.Node{
		Type: html.ElementNode,
		Data: "script",
		Attr: []html.Attribute{
			{Key: "type", Val: "text/javascript"},
		},
	}

	var scriptBuf bytes.Buffer
	err = a.scriptTmpl.Execute(&scriptBuf, struct {
		Domain      string
		IsSubdomain bool
	}{
		Domain:      html.EscapeString(a.portalDomain),
		IsSubdomain: a.portalIsSubdomain,
	})
	if err != nil {
		return content
	}

	scriptText := &html.Node{
		Type: html.TextNode,
		Data: scriptBuf.String(),
	}
	script.AppendChild(scriptText)
	head.AppendChild(script)

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return content // fallback to original if rendering fails
	}

	return buf.Bytes()
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
func (fi *appFileInfo) Sys() interface{}   { return nil }
