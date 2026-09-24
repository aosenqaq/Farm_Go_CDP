package lanaccess

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	avatarProxyPathPrefix = "/farm-avatars/"
	maxAvatarBytes        = 2 << 20
	maxAvatarRedirects    = 3
)

var (
	errAvatarNotFound        = errors.New("avatar token not found")
	errAvatarUnauthorized    = errors.New("avatar token is not owned by this session")
	errAvatarInvalidResponse = errors.New("avatar upstream response is invalid")
)

type avatarProxy struct {
	mu          sync.Mutex
	entries     map[string]avatarEntry
	now         func() time.Time
	client      *http.Client
	lookupNetIP func(context.Context, string, string) ([]netip.Addr, error)
}

type avatarEntry struct {
	sourceURL string
	sessionID string
	expiresAt time.Time
}

type avatarImage struct {
	contentType string
	bytes       []byte
}

func newAvatarProxy(clock Clock) *avatarProxy {
	if clock == nil {
		clock = systemClock{}
	}
	return &avatarProxy{
		entries:     make(map[string]avatarEntry),
		now:         clock.Now,
		client:      &http.Client{Timeout: 8 * time.Second},
		lookupNetIP: net.DefaultResolver.LookupNetIP,
	}
}

func (p *avatarProxy) rewriteJSON(session Session, value any) any {
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var copyValue any
	if err := json.Unmarshal(raw, &copyValue); err != nil {
		return value
	}
	rewriteAvatarURLs(copyValue, func(source string) string {
		return p.register(session, source)
	})
	return copyValue
}

func rewriteAvatarURLs(value any, replace func(string) string) {
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			if key == "avatarUrl" {
				source, ok := child.(string)
				if !ok {
					item[key] = ""
					continue
				}
				item[key] = replace(source)
				continue
			}
			rewriteAvatarURLs(child, replace)
		}
	case []any:
		for _, child := range item {
			rewriteAvatarURLs(child, replace)
		}
	}
}

func (p *avatarProxy) register(session Session, source string) string {
	if session.ID == "" || !p.now().Before(session.ExpiresAt) {
		return ""
	}
	if _, err := parseAvatarSource(source); err != nil {
		return ""
	}
	token, err := randomToken()
	if err != nil {
		return ""
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.pruneExpiredLocked(p.now())
	p.entries[token] = avatarEntry{sourceURL: source, sessionID: session.ID, expiresAt: session.ExpiresAt}
	return avatarProxyPathPrefix + token
}

func (p *avatarProxy) fetch(ctx context.Context, session Session, token string) (avatarImage, error) {
	now := p.now()
	p.mu.Lock()
	p.pruneExpiredLocked(now)
	entry, found := p.entries[token]
	p.mu.Unlock()
	if !found {
		return avatarImage{}, errAvatarNotFound
	}
	if session.ID != entry.sessionID || !now.Before(session.ExpiresAt) {
		return avatarImage{}, errAvatarUnauthorized
	}
	if err := p.validatePublicHTTPSURL(ctx, entry.sourceURL); err != nil {
		return avatarImage{}, errAvatarInvalidResponse
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, entry.sourceURL, nil)
	if err != nil {
		return avatarImage{}, errAvatarInvalidResponse
	}
	client := *p.client
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) > maxAvatarRedirects {
			return errAvatarInvalidResponse
		}
		return p.validatePublicHTTPSURL(request.Context(), request.URL.String())
	}
	response, err := client.Do(request)
	if err != nil {
		return avatarImage{}, errAvatarInvalidResponse
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return avatarImage{}, errAvatarInvalidResponse
	}
	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || !isAvatarContentType(contentType) {
		return avatarImage{}, errAvatarInvalidResponse
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAvatarBytes+1))
	if err != nil || len(body) == 0 || len(body) > maxAvatarBytes {
		return avatarImage{}, errAvatarInvalidResponse
	}
	return avatarImage{contentType: contentType, bytes: body}, nil
}

func (p *avatarProxy) pruneExpiredLocked(now time.Time) {
	for token, entry := range p.entries {
		if !now.Before(entry.expiresAt) {
			delete(p.entries, token)
		}
	}
}

func parseAvatarSource(raw string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Hostname() == "" {
		return nil, errAvatarInvalidResponse
	}
	return parsed, nil
}

func (p *avatarProxy) validatePublicHTTPSURL(ctx context.Context, raw string) error {
	parsed, err := parseAvatarSource(raw)
	if err != nil {
		return err
	}
	addresses, err := p.lookupNetIP(ctx, "ip", parsed.Hostname())
	if err != nil || len(addresses) == 0 {
		return errAvatarInvalidResponse
	}
	for _, address := range addresses {
		if !isPublicAvatarAddress(address) {
			return errAvatarInvalidResponse
		}
	}
	return nil
}

func isPublicAvatarAddress(address netip.Addr) bool {
	return address.IsGlobalUnicast() && !address.IsPrivate() && !address.IsLoopback() && !address.IsLinkLocalUnicast() && !address.IsMulticast() && !address.IsUnspecified()
}

func isAvatarContentType(contentType string) bool {
	switch strings.ToLower(contentType) {
	case "image/avif", "image/gif", "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}
