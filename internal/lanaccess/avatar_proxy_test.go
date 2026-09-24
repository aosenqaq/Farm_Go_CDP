package lanaccess

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestAvatarProxyBindsRegisteredURLsToTheOwningSession(t *testing.T) {
	proxy := newAvatarProxy(systemClock{})
	owner := Session{ID: "owner", ExpiresAt: time.Now().Add(time.Hour)}
	path := proxy.register(owner, "https://avatar.example/account.png")
	if !strings.HasPrefix(path, avatarProxyPathPrefix) {
		t.Fatalf("avatar path = %q", path)
	}

	_, err := proxy.fetch(context.Background(), Session{ID: "other", ExpiresAt: owner.ExpiresAt}, strings.TrimPrefix(path, avatarProxyPathPrefix))
	if !errors.Is(err, errAvatarUnauthorized) {
		t.Fatalf("other session error = %v", err)
	}
}

func TestAvatarProxyFetchAcceptsOnlyBoundedImageResponses(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/image":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png"))
		case "/text":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("not an image"))
		default:
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(make([]byte, maxAvatarBytes+1))
		}
	}))
	defer upstream.Close()

	proxy := newAvatarProxy(systemClock{})
	proxy.client = upstream.Client()
	proxy.lookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{
			netip.MustParseAddr("8.8.8.8"),
			netip.MustParseAddr("2606:4700:4700::1111"),
		}, nil
	}
	owner := Session{ID: "owner", ExpiresAt: time.Now().Add(time.Hour)}
	for _, tc := range []struct {
		path string
		want error
	}{
		{path: "/image"},
		{path: "/text", want: errAvatarInvalidResponse},
		{path: "/large", want: errAvatarInvalidResponse},
	} {
		t.Run(tc.path, func(t *testing.T) {
			path := proxy.register(owner, upstream.URL+tc.path)
			image, err := proxy.fetch(context.Background(), owner, strings.TrimPrefix(path, avatarProxyPathPrefix))
			if !errors.Is(err, tc.want) {
				t.Fatalf("fetch error = %v, want %v", err, tc.want)
			}
			if tc.want == nil && (image.contentType != "image/png" || string(image.bytes) != "png") {
				t.Fatalf("image = %#v", image)
			}
		})
	}
}
