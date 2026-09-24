package lanaccess

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Iterations  uint32 = 3
	argon2Memory      uint32 = 64 * 1024
	argon2Parallelism uint8  = 4
	argon2KeyLength   uint32 = 32
	passwordSaltBytes        = 16
	tokenBytes               = 32
	sessionLifetime          = 12 * time.Hour
	lanLockDuration          = 15 * time.Minute
)

var (
	ErrPasswordTooShort = errors.New("LAN access password must contain at least 12 characters")
	ErrInvalidPassword  = errors.New("invalid LAN access password")
	ErrLocked           = errors.New("LAN access login is temporarily locked")
)

// Clock makes session expiration and lockouts deterministic in tests.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

type Session struct {
	ID        string
	CSRF      string
	ExpiresAt time.Time
}

type RetryError struct {
	After time.Duration
}

func (e RetryError) Error() string {
	return fmt.Sprintf("retry login after %s", e.After)
}

type Authenticator struct {
	mu sync.Mutex

	clock      Clock
	password   string
	generation uint64
	sessions   map[string]storedSession
	failures   map[string]loginFailure
}

type storedSession struct {
	csrf      string
	expiresAt time.Time
}

type loginFailure struct {
	count       int
	lockedUntil time.Time
}

func NewAuthenticator(clock Clock) *Authenticator {
	if clock == nil {
		clock = systemClock{}
	}
	return &Authenticator{
		clock:    clock,
		sessions: make(map[string]storedSession),
		failures: make(map[string]loginFailure),
	}
}

func HashPassword(password string) (string, error) {
	if utf8.RuneCountInString(password) < 12 {
		return "", ErrPasswordTooShort
	}

	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLength)
	return strings.Join([]string{
		"argon2id",
		"v=1",
		"m=65536,t=3,p=4",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	}, "$"), nil
}

func VerifyPassword(encodedHash, password string) bool {
	salt, expectedKey, ok := parsePasswordHash(encodedHash)
	if !ok {
		return false
	}
	actualKey := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLength)
	return subtle.ConstantTimeCompare(expectedKey, actualKey) == 1
}

func (a *Authenticator) SetPasswordHash(hash string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.password = hash
	a.revokeAllLocked()
}

func (a *Authenticator) RevokeAll() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revokeAllLocked()
}

func (a *Authenticator) Login(password, remoteAddr string, mode Mode) (Session, error) {
	now := a.clock.Now()
	key := loginKey(mode, remoteAddr)

	a.mu.Lock()
	if a.isLANLockedLocked(key, mode, now) {
		a.mu.Unlock()
		return Session{}, ErrLocked
	}
	hash := a.password
	generation := a.generation
	a.mu.Unlock()

	if !VerifyPassword(hash, password) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if generation != a.generation {
			return Session{}, ErrInvalidPassword
		}
		return Session{}, a.recordFailureLocked(key, mode, now)
	}

	sessionID, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrf, err := randomToken()
	if err != nil {
		return Session{}, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if generation != a.generation || a.isLANLockedLocked(key, mode, now) {
		return Session{}, ErrLocked
	}

	delete(a.failures, key)
	session := Session{ID: sessionID, CSRF: csrf, ExpiresAt: now.Add(sessionLifetime)}
	a.sessions[session.ID] = storedSession{csrf: session.CSRF, expiresAt: session.ExpiresAt}
	return session, nil
}

func (a *Authenticator) Authorize(sessionID, csrf string) bool {
	now := a.clock.Now()
	a.mu.Lock()
	defer a.mu.Unlock()

	session, ok := a.sessions[sessionID]
	if !ok {
		return false
	}
	if !now.Before(session.expiresAt) {
		delete(a.sessions, sessionID)
		return false
	}
	return subtle.ConstantTimeCompare([]byte(session.csrf), []byte(csrf)) == 1
}

func (a *Authenticator) revokeAllLocked() {
	a.generation++
	a.sessions = make(map[string]storedSession)
}

func (a *Authenticator) isLANLockedLocked(key string, mode Mode, now time.Time) bool {
	if mode != ModeLAN {
		return false
	}
	failure, ok := a.failures[key]
	if !ok || failure.lockedUntil.IsZero() {
		return false
	}
	if !now.Before(failure.lockedUntil) {
		delete(a.failures, key)
		return false
	}
	return true
}

func (a *Authenticator) recordFailureLocked(key string, mode Mode, now time.Time) error {
	failure := a.failures[key]
	failure.count++
	if mode == ModeLAN {
		if failure.count == 5 {
			failure.lockedUntil = now.Add(lanLockDuration)
		}
		a.failures[key] = failure
		return ErrInvalidPassword
	}

	a.failures[key] = failure
	return RetryError{After: tunnelRetryAfter(failure.count)}
}

func parsePasswordHash(encodedHash string) ([]byte, []byte, bool) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=1" || parts[2] != "m=65536,t=3,p=4" {
		return nil, nil, false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(salt) != passwordSaltBytes {
		return nil, nil, false
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(key) != int(argon2KeyLength) {
		return nil, nil, false
	}
	return salt, key, true
}

func randomToken() (string, error) {
	token := make([]byte, tokenBytes)
	if _, err := rand.Read(token); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(token), nil
}

func loginKey(mode Mode, remoteAddr string) string {
	if mode == ModeTunnel {
		return "tunnel"
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		addr, parseErr := netip.ParseAddr(host)
		if parseErr == nil && addr.Is4() {
			return "lan:" + addr.String()
		}
	}
	return "lan:" + remoteAddr
}

func tunnelRetryAfter(attempt int) time.Duration {
	seconds := 1
	for attempt > 1 && seconds < 60 {
		seconds *= 2
		attempt--
	}
	if seconds > 60 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

var _ Clock = systemClock{}
