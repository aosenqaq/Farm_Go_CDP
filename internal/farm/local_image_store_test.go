package farm

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalImageStoreReusesCachedBytesForRegisteredPath(t *testing.T) {
	root := t.TempDir()
	imagePath := filepath.Join(root, "plant_images", "default", "400.png")
	writeTinyPNG(t, imagePath)
	store := NewLocalImageStore(root)
	url := store.URLFor(imagePath)
	if !strings.HasPrefix(url, "/farm-assets/") {
		t.Fatalf("expected local asset URL, got %q", url)
	}

	first := httptest.NewRecorder()
	store.ServeHTTP(first, httptest.NewRequest(http.MethodGet, url, nil))
	if first.Code != http.StatusOK || first.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("unexpected first response: code=%d type=%q", first.Code, first.Header().Get("Content-Type"))
	}
	if first.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("unexpected cache control %q", first.Header().Get("Cache-Control"))
	}

	if err := os.WriteFile(imagePath, []byte("changed"), 0o644); err != nil {
		t.Fatalf("rewrite cached image: %v", err)
	}
	second := httptest.NewRecorder()
	store.ServeHTTP(second, httptest.NewRequest(http.MethodGet, url, nil))
	if second.Code != http.StatusOK || string(second.Body.Bytes()) != string(first.Body.Bytes()) {
		t.Fatalf("expected cached bytes to be reused: first=%q second=%q", first.Body.Bytes(), second.Body.Bytes())
	}
}

func TestLocalImageStoreRejectsUnknownAndEscapedPaths(t *testing.T) {
	store := NewLocalImageStore(t.TempDir())
	for _, path := range []string{"/farm-assets/missing", "/farm-assets/../Plant.json"} {
		response := httptest.NewRecorder()
		store.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("expected not found for %q, got %d", path, response.Code)
		}
	}
}
