package lanaccess

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword(%q) error = %v", password, err)
	}
	return hash
}

func TestHashPasswordRejectsPasswordsShorterThanTwelveCharacters(t *testing.T) {
	if _, err := HashPassword("12345678901"); err == nil {
		t.Fatal("HashPassword accepted an 11-character password")
	}

	if _, err := HashPassword("123456789012"); err != nil {
		t.Fatalf("HashPassword rejected a 12-character password: %v", err)
	}
}

func TestHashPasswordUsesVerifiableArgon2idEncodingWithRandomSalt(t *testing.T) {
	password := "correct horse battery staple"
	first := mustHash(t, password)
	second := mustHash(t, password)
	if first == second {
		t.Fatal("HashPassword reused salt")
	}

	for _, hash := range []string{first, second} {
		parts := strings.Split(hash, "$")
		if len(parts) != 5 {
			t.Fatalf("hash has %d parts, want 5: %q", len(parts), hash)
		}
		if parts[0] != "argon2id" || parts[1] != "v=1" || parts[2] != "m=65536,t=3,p=4" {
			t.Fatalf("hash parameters = %q, %q, %q", parts[0], parts[1], parts[2])
		}
		salt, err := base64.RawStdEncoding.DecodeString(parts[3])
		if err != nil || len(salt) != 16 {
			t.Fatalf("salt = %d bytes, decode error = %v", len(salt), err)
		}
		key, err := base64.RawStdEncoding.DecodeString(parts[4])
		if err != nil || len(key) != 32 {
			t.Fatalf("key = %d bytes, decode error = %v", len(key), err)
		}
		if !VerifyPassword(hash, password) {
			t.Fatal("VerifyPassword rejected its HashPassword output")
		}
		if VerifyPassword(hash, "wrong password") {
			t.Fatal("VerifyPassword accepted the wrong password")
		}
	}

	if VerifyPassword("argon2id$v=1$m=65536,t=3,p=4$invalid$invalid", password) {
		t.Fatal("VerifyPassword accepted malformed encoding")
	}
}

func TestAuthenticatorCreatesRandomSessionAndCSRFForTwelveHours(t *testing.T) {
	clock := newFakeClock()
	auth := NewAuthenticator(clock)
	auth.SetPasswordHash(mustHash(t, "correct horse battery staple"))

	first, err := auth.Login("correct horse battery staple", "192.168.1.5:1000", ModeLAN)
	if err != nil {
		t.Fatalf("first Login() error = %v", err)
	}
	second, err := auth.Login("correct horse battery staple", "192.168.1.5:1001", ModeLAN)
	if err != nil {
		t.Fatalf("second Login() error = %v", err)
	}
	if first.ID == second.ID || first.CSRF == second.CSRF {
		t.Fatal("Login reused a session or CSRF token")
	}
	for _, token := range []string{first.ID, first.CSRF} {
		decoded, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil || len(decoded) != 32 {
			t.Fatalf("token = %d bytes, decode error = %v", len(decoded), err)
		}
	}
	if !auth.Authorize(first.ID, first.CSRF) {
		t.Fatal("fresh session was not authorized")
	}

	clock.Advance(12 * time.Hour)
	if auth.Authorize(first.ID, first.CSRF) {
		t.Fatal("session remained authorized after 12 hours")
	}
}

func TestAuthenticatorRevokesAllSessionsAfterPasswordChange(t *testing.T) {
	auth := NewAuthenticator(newFakeClock())
	auth.SetPasswordHash(mustHash(t, "correct horse battery staple"))
	first, err := auth.Login("correct horse battery staple", "192.168.1.5:1000", ModeLAN)
	if err != nil {
		t.Fatalf("first Login() error = %v", err)
	}
	second, err := auth.Login("correct horse battery staple", "192.168.1.5:1001", ModeLAN)
	if err != nil {
		t.Fatalf("second Login() error = %v", err)
	}

	auth.SetPasswordHash(mustHash(t, "new correct horse battery staple"))
	if auth.Authorize(first.ID, first.CSRF) || auth.Authorize(second.ID, second.CSRF) {
		t.Fatal("SetPasswordHash left an old session authorized")
	}
}

func TestAuthenticatorRevokeAllRevokesEverySession(t *testing.T) {
	auth := NewAuthenticator(newFakeClock())
	auth.SetPasswordHash(mustHash(t, "correct horse battery staple"))
	session, err := auth.Login("correct horse battery staple", "192.168.1.5:1000", ModeLAN)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	auth.RevokeAll()
	if auth.Authorize(session.ID, session.CSRF) {
		t.Fatal("RevokeAll left a session authorized")
	}
}

func TestAuthenticatorLocksLANIPv4AfterFiveFailures(t *testing.T) {
	clock := newFakeClock()
	auth := NewAuthenticator(clock)
	auth.SetPasswordHash(mustHash(t, "correct horse battery staple"))

	for attempt := 0; attempt < 5; attempt++ {
		_, err := auth.Login("wrong", "192.168.1.5:"+strconv.Itoa(1000+attempt), ModeLAN)
		if !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("attempt %d error = %v, want ErrInvalidPassword", attempt+1, err)
		}
	}
	if _, err := auth.Login("wrong", "192.168.1.5:9999", ModeLAN); !errors.Is(err, ErrLocked) {
		t.Fatalf("sixth failure error = %v, want ErrLocked", err)
	}
	if _, err := auth.Login("correct horse battery staple", "192.168.1.5:9999", ModeLAN); !errors.Is(err, ErrLocked) {
		t.Fatalf("locked IP accepted correct password: %v", err)
	}

	clock.Advance(15 * time.Minute)
	if _, err := auth.Login("correct horse battery staple", "192.168.1.5:9999", ModeLAN); err != nil {
		t.Fatalf("Login() after lock expiry error = %v", err)
	}
}

func TestAuthenticatorUsesTunnelKeyForProgressiveBackoff(t *testing.T) {
	clock := newFakeClock()
	auth := NewAuthenticator(clock)
	auth.SetPasswordHash(mustHash(t, "correct horse battery staple"))

	for attempt, want := range []time.Duration{
		time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		32 * time.Second,
		60 * time.Second,
		60 * time.Second,
	} {
		_, err := auth.Login("wrong", "127.0.0.1:"+strconv.Itoa(1000+attempt), ModeTunnel)
		var retry RetryError
		if !errors.As(err, &retry) || retry.After != want {
			t.Fatalf("attempt %d retry = %#v, want %s", attempt+1, retry, want)
		}
		clock.Advance(want)
	}
}

func TestAuthenticatorSerializesConcurrentAuthorizationAndRevocation(t *testing.T) {
	auth := NewAuthenticator(newFakeClock())
	auth.SetPasswordHash(mustHash(t, "correct horse battery staple"))
	session, err := auth.Login("correct horse battery staple", "192.168.1.5:1000", ModeLAN)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				auth.Authorize(session.ID, session.CSRF)
			}
		}()
	}
	for i := 0; i < 100; i++ {
		auth.RevokeAll()
	}
	wg.Wait()
}
