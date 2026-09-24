package lanaccess

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookieName = "farm_go_lan"
	// Automation settings can include cached crop metadata and exceed 1 MiB.
	maxJSONBodyBytes      = 8 << 20
	sessionCookieAge      = 12 * 60 * 60
	remoteBootstrapPath   = "/farm-go-remote.js"
	remoteBootstrapScript = "window.__FARM_GO_REMOTE__=true;\n"
)

var ErrMethodNotAllowed = errors.New("LAN RPC method is not allowed")

// RPCDispatcher exposes only methods explicitly allowed by the application.
type RPCDispatcher interface {
	Dispatch(context.Context, string, []json.RawMessage) (any, error)
}

type Options struct {
	Config     Config
	Auth       *Authenticator
	Assets     fs.FS
	GameAssets http.Handler
	RPC        RPCDispatcher
	Poll       http.Handler
}

type server struct {
	config     Config
	auth       *Authenticator
	assets     fs.FS
	gameAssets http.Handler
	static     http.Handler
	rpc        RPCDispatcher
	poll       http.Handler
	avatars    *avatarProxy
}

// NewServer creates the HTTP boundary used by LAN and tunnel clients.
func NewServer(options Options) http.Handler {
	auth := options.Auth
	if auth == nil {
		auth = NewAuthenticator(nil)
		if options.Config.PasswordHash != "" {
			auth.SetPasswordHash(options.Config.PasswordHash)
		}
	}

	static := http.NotFoundHandler()
	if options.Assets != nil {
		static = http.FileServer(http.FS(options.Assets))
	}

	return &server{
		config:     options.Config,
		auth:       auth,
		assets:     options.Assets,
		gameAssets: options.GameAssets,
		static:     static,
		rpc:        options.RPC,
		poll:       options.Poll,
		avatars:    newAvatarProxy(auth.clock),
	}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.setSecurityHeaders(w)
	w.Header().Set("Cache-Control", "no-store")

	if !AllowsRemoteAddr(s.config.Mode, r.RemoteAddr) {
		s.writeError(w, http.StatusForbidden, "remote address is not allowed")
		return
	}

	switch {
	case r.URL.Path == remoteBootstrapPath:
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, http.MethodGet)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = io.WriteString(w, s.remoteBootstrapScript())
	case r.URL.Path == "/api/auth/login":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w, http.MethodPost)
			return
		}
		s.login(w, r)
	case r.URL.Path == "/api/auth/logout":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w, http.MethodPost)
			return
		}
		s.logout(w, r)
	case r.URL.Path == "/api/auth/session":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w, http.MethodGet)
			return
		}
		s.session(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/rpc/"):
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w, http.MethodPost)
			return
		}
		s.rpcCall(w, r)
	case strings.HasPrefix(r.URL.Path, "/farm-api/poll/"):
		s.servePoll(w, r)
	case strings.HasPrefix(r.URL.Path, "/farm-assets/"):
		s.serveGameAsset(w, r)
	case strings.HasPrefix(r.URL.Path, avatarProxyPathPrefix):
		s.serveAvatar(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/"):
		s.writeError(w, http.StatusNotFound, "API route not found")
	default:
		s.serveStatic(w, r)
	}
}

func (s *server) serveGameAsset(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/farm-assets/")
	if r.Method != http.MethodGet || s.gameAssets == nil || id == "" || strings.ContainsAny(id, "/\\") {
		http.NotFound(w, r)
		return
	}
	s.gameAssets.ServeHTTP(w, r)
}

func (s *server) serveAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, http.MethodGet)
		return
	}
	session, ok := s.sessionFromRequest(r)
	if !ok {
		s.writeError(w, http.StatusUnauthorized, "authorization required")
		return
	}
	token := strings.TrimPrefix(r.URL.Path, avatarProxyPathPrefix)
	if token == "" || strings.ContainsAny(token, "/\\") {
		s.writeError(w, http.StatusNotFound, "avatar not found")
		return
	}
	image, err := s.avatars.fetch(r.Context(), session, token)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "avatar not found")
		return
	}
	w.Header().Set("Content-Type", image.contentType)
	_, _ = w.Write(image.bytes)
}

func (s *server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if (r.URL.Path == "/" || r.URL.Path == "/index.html") && s.assets != nil {
		index, err := fs.ReadFile(s.assets, "index.html")
		if err == nil {
			index = bytes.Replace(index, []byte("</head>"), []byte("<script src=\""+remoteBootstrapPath+"\"></script></head>"), 1)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(index)
			return
		}
	}
	s.static.ServeHTTP(w, r)
}

func (s *server) remoteBootstrapScript() string {
	script := remoteBootstrapScript
	if s.config.Mode == ModeTunnel {
		script += "window.__FARM_GO_TUNNEL__=true;\n"
	}
	return script
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		s.writeError(w, http.StatusBadRequest, "invalid login request")
		return
	}

	session, err := s.auth.Login(request.Password, r.RemoteAddr, s.config.Mode)
	if err != nil {
		var retry RetryError
		switch {
		case errors.Is(err, ErrInvalidPassword):
			s.writeError(w, http.StatusUnauthorized, "invalid credentials")
		case errors.Is(err, ErrLocked):
			s.writeError(w, http.StatusTooManyRequests, "login temporarily unavailable")
		case errors.As(err, &retry):
			w.Header().Set("Retry-After", retryAfterSeconds(retry.After))
			s.writeError(w, http.StatusTooManyRequests, "login temporarily unavailable")
		default:
			s.writeError(w, http.StatusInternalServerError, "login failed")
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID,
		Path:     "/",
		MaxAge:   sessionCookieAge,
		HttpOnly: true,
		Secure:   s.config.Mode == ModeTunnel,
		SameSite: http.SameSiteStrictMode,
	})
	s.writeJSON(w, http.StatusOK, sessionResponse{Authorized: true, CSRFToken: session.CSRF})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSessionAndCSRF(w, r)
	if !ok {
		return
	}

	s.auth.mu.Lock()
	delete(s.auth.sessions, session.ID)
	s.auth.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.config.Mode == ModeTunnel,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) session(w http.ResponseWriter, r *http.Request) {
	session, ok := s.sessionFromRequest(r)
	if !ok {
		s.writeJSON(w, http.StatusOK, sessionResponse{})
		return
	}
	s.writeJSON(w, http.StatusOK, sessionResponse{Authorized: true, CSRFToken: session.CSRF})
}

func (s *server) rpcCall(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSessionAndCSRF(w, r)
	if !ok {
		return
	}

	var request struct {
		Args json.RawMessage `json:"args"`
	}
	if !decodeJSON(w, r, &request) {
		s.writeError(w, http.StatusBadRequest, "invalid RPC request")
		return
	}

	rawArgs := bytes.TrimSpace(request.Args)
	if len(rawArgs) == 0 || rawArgs[0] != '[' {
		s.writeError(w, http.StatusBadRequest, "RPC args must be an array")
		return
	}
	var args []json.RawMessage
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		s.writeError(w, http.StatusBadRequest, "RPC args must be an array")
		return
	}

	method := strings.TrimPrefix(r.URL.Path, "/api/rpc/")
	if method == "" || s.rpc == nil {
		s.writeError(w, http.StatusNotFound, "RPC method not found")
		return
	}
	result, err := s.rpc.Dispatch(r.Context(), method, args)
	if err != nil {
		if errors.Is(err, ErrMethodNotAllowed) {
			s.writeError(w, http.StatusNotFound, "RPC method not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "RPC request failed")
		return
	}
	s.writeJSON(w, http.StatusOK, s.avatars.rewriteJSON(session, result))
}

func (s *server) servePoll(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessionFromRequest(r); !ok {
		s.writeError(w, http.StatusUnauthorized, "authorization required")
		return
	}
	if s.poll == nil {
		s.writeError(w, http.StatusNotFound, "poll route not found")
		return
	}
	s.poll.ServeHTTP(&apiResponseWriter{ResponseWriter: w, server: s}, r)
}

func (s *server) requireSessionAndCSRF(w http.ResponseWriter, r *http.Request) (Session, bool) {
	session, ok := s.sessionFromRequest(r)
	if !ok {
		s.writeError(w, http.StatusUnauthorized, "authorization required")
		return Session{}, false
	}
	if r.Header.Get("Origin") != s.requestOrigin(r) {
		s.writeError(w, http.StatusForbidden, "invalid request origin")
		return Session{}, false
	}
	if subtle.ConstantTimeCompare([]byte(session.CSRF), []byte(r.Header.Get("X-Farm-Go-CSRF"))) != 1 || !s.auth.Authorize(session.ID, r.Header.Get("X-Farm-Go-CSRF")) {
		s.writeError(w, http.StatusForbidden, "invalid CSRF token")
		return Session{}, false
	}
	return session, true
}

func (s *server) sessionFromRequest(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return Session{}, false
	}

	s.auth.mu.Lock()
	defer s.auth.mu.Unlock()
	stored, ok := s.auth.sessions[cookie.Value]
	if !ok {
		return Session{}, false
	}
	if !s.auth.clock.Now().Before(stored.expiresAt) {
		delete(s.auth.sessions, cookie.Value)
		return Session{}, false
	}
	return Session{ID: cookie.Value, CSRF: stored.csrf, ExpiresAt: stored.expiresAt}, true
}

func (s *server) methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (s *server) setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; frame-ancestors 'self'; form-action 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
}

type apiResponseWriter struct {
	http.ResponseWriter
	server *server
}

func (w *apiResponseWriter) WriteHeader(status int) {
	w.enforceHeaders()
	w.ResponseWriter.WriteHeader(status)
}

func (w *apiResponseWriter) Write(body []byte) (int, error) {
	w.enforceHeaders()
	return w.ResponseWriter.Write(body)
}

func (w *apiResponseWriter) enforceHeaders() {
	w.server.setSecurityHeaders(w.ResponseWriter)
	headers := w.Header()
	headers.Set("Cache-Control", "no-store")
	for name := range headers {
		if strings.HasPrefix(name, "Access-Control-") {
			headers.Del(name)
		}
	}
}

type sessionResponse struct {
	Authorized bool   `json:"authorized"`
	CSRFToken  string `json:"csrfToken"`
}

func (s *server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

func (s *server) writeJSON(w http.ResponseWriter, status int, value any) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		body.Reset()
		body.WriteString("{\"error\":\"response encoding failed\"}\n")
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return false
	}
	return true
}

func (s *server) requestOrigin(r *http.Request) string {
	if s.config.Mode == ModeTunnel {
		return "https://" + r.Host
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func retryAfterSeconds(after time.Duration) string {
	if after <= 0 {
		return "1"
	}
	seconds := int64(after / time.Second)
	if after%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}
