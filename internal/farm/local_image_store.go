package farm

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const localImageAssetPrefix = "/farm-assets/"

type localImageEntry struct {
	path  string
	mime  string
	bytes []byte
}

type LocalImageStore struct {
	root   string
	mu     sync.RWMutex
	byID   map[string]*localImageEntry
	byPath map[string]string
}

func NewLocalImageStore(root string) *LocalImageStore {
	return &LocalImageStore{
		root:   canonicalImageRoot(root),
		byID:   map[string]*localImageEntry{},
		byPath: map[string]string{},
	}
}

func (s *LocalImageStore) URLFor(path string) string {
	canonical, ok := s.canonicalPath(path)
	if !ok {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, found := s.byPath[canonical]; found {
		return localImageAssetPrefix + id
	}
	digest := sha256.Sum256([]byte(canonical))
	id := hex.EncodeToString(digest[:])
	s.byPath[canonical] = id
	s.byID[id] = &localImageEntry{path: canonical, mime: imageMIMEType(canonical)}
	return localImageAssetPrefix + id
}

func (s *LocalImageStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, localImageAssetPrefix)
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	s.mu.RLock()
	entry := s.byID[id]
	s.mu.RUnlock()
	if entry == nil {
		http.NotFound(w, r)
		return
	}

	s.mu.Lock()
	if entry.bytes == nil {
		bytes, err := os.ReadFile(entry.path)
		if err != nil || len(bytes) == 0 {
			s.mu.Unlock()
			http.NotFound(w, r)
			return
		}
		entry.bytes = bytes
	}
	bytes := entry.bytes
	mime := entry.mime
	s.mu.Unlock()

	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(bytes)
}

func (s *LocalImageStore) canonicalPath(path string) (string, bool) {
	if s.root == "" || strings.TrimSpace(path) == "" {
		return "", false
	}
	candidate, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	candidate = filepath.Clean(candidate)
	if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
		candidate = resolved
	}
	relative, err := filepath.Rel(s.root, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}
	return candidate, true
}

func canonicalImageRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	canonical, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(canonical); err == nil {
		canonical = resolved
	}
	return filepath.Clean(canonical)
}

func imageMIMEType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

var gameConfigLocalImages = struct {
	sync.Mutex
	stores map[string]*LocalImageStore
}{stores: map[string]*LocalImageStore{}}

func gameConfigLocalImageURL(root string, path string) string {
	canonicalRoot := canonicalImageRoot(root)
	if canonicalRoot == "" {
		return ""
	}
	gameConfigLocalImages.Lock()
	store := gameConfigLocalImages.stores[canonicalRoot]
	if store == nil {
		store = NewLocalImageStore(canonicalRoot)
		gameConfigLocalImages.stores[canonicalRoot] = store
	}
	gameConfigLocalImages.Unlock()
	return store.URLFor(path)
}

func ServeLocalGameConfigImage(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, localImageAssetPrefix) {
		return false
	}
	gameConfigLocalImages.Lock()
	stores := make([]*LocalImageStore, 0, len(gameConfigLocalImages.stores))
	for _, store := range gameConfigLocalImages.stores {
		stores = append(stores, store)
	}
	gameConfigLocalImages.Unlock()
	for _, store := range stores {
		store.mu.RLock()
		_, found := store.byID[strings.TrimPrefix(r.URL.Path, localImageAssetPrefix)]
		store.mu.RUnlock()
		if found {
			store.ServeHTTP(w, r)
			return true
		}
	}
	http.NotFound(w, r)
	return true
}
