package shop

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

type staticFile struct {
	body        *Body
	contentType string
	cache       string
}

// staticSite serves a prebuilt SPA from memory. Every file is read and
// gzip-compressed once at startup; unknown paths fall back to index.html.
type staticSite struct {
	files map[string]*staticFile
	index *staticFile
}

func newStaticSite(fsys fs.FS) (*staticSite, error) {
	s := &staticSite{files: map[string]*staticFile{}}
	if fsys == nil {
		return s, nil
	}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		ct := mime.TypeByExtension(path.Ext(p))
		if ct == "" {
			ct = http.DetectContentType(data)
		}
		cache := "public, max-age=3600"
		switch {
		case strings.HasPrefix(p, "assets/"): // Vite content-hashed output
			cache = "public, max-age=31536000, immutable"
		case p == "index.html":
			cache = "no-cache"
		}
		s.files["/"+p] = &staticFile{body: NewBody(data), contentType: ct, cache: cache}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.index = s.files["/index.html"]
	return s, nil
}

func (s *staticSite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	f := s.files[r.URL.Path]
	if f == nil {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			http.NotFound(w, r) // a missing hashed asset must not get HTML
			return
		}
		f = s.index
	}
	if f == nil {
		http.Error(w, "UI not built (run `make web`)", http.StatusNotFound)
		return
	}
	serveBody(w, r, f.body, f.contentType, f.cache)
}
