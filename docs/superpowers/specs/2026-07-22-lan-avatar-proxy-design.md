# LAN WebUI Avatar Proxy Design

## Goal

Show real account avatars in the LAN WebUI without weakening its existing same-origin Content Security Policy. The repair covers the sidebar account panel and account identity dialog. Friend avatar URLs returned by LAN RPC calls are also made usable, but this change does not add friend-avatar UI where none exists today.

## Root Cause

Runtime account and social payloads already provide external `avatarUrl` values. The LAN server sends `Content-Security-Policy: default-src 'self'`, so the browser rejects those cross-origin image requests. `FallbackImage` then replaces the failed image with `/logo.png`.

## Design

The LAN server owns an in-memory avatar registry. After an authenticated RPC dispatch succeeds, the server serializes the response as JSON and recursively replaces every `avatarUrl` string containing a valid HTTPS URL with a same-origin opaque path:

```
authenticated RPC result -> avatarUrl rewrite -> /farm-avatars/<token> -> browser image request
```

Each generated token records its source URL, expiry, and the session ID that received it. The original URL is never sent to the browser. Existing React image components consume the rewritten path without source or style changes.

`GET /farm-avatars/<token>` requires the current authenticated session to own the token. The handler fetches the registered avatar source and streams only a valid image response. It preserves the server's existing security headers and `Cache-Control: no-store` behavior.

## Security Boundaries

- The avatar route accepts an opaque token only; it never accepts a caller-provided URL.
- Tokens are bound to a single authenticated LAN session and expire with that session window.
- Sources must use HTTPS and resolve only to public addresses. Redirect targets undergo the same validation.
- The upstream response is size-limited and must be an image. Invalid source URLs, non-image responses, redirects to unsafe targets, oversized responses, and upstream failures are rejected.
- CSP remains `default-src 'self'`; CORS is not added and desktop rendering is unchanged.

## Failure Handling

If a source cannot be fetched or validated, the avatar route returns a non-success response. The existing `FallbackImage` behavior then selects `/logo.png`; accounts with no avatar URL continue using their icon fallback.

## Tests

Add LAN server tests proving that:

- nested account and social `avatarUrl` values in an authenticated RPC response become same-origin avatar paths;
- the owning authenticated session can load a proxied image;
- anonymous and different authenticated sessions cannot load that token;
- invalid, non-image, and oversized upstream responses are rejected;
- existing LAN server security and RPC tests continue to pass.

Run focused LAN tests, the full Go suite, and the existing frontend test/build commands. No frontend component change is expected for this repair.
