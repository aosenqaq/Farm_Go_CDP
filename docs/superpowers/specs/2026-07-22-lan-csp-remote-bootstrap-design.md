# LAN WebUI CSP Remote Bootstrap Design

## Goal

Make the LAN WebUI enter its browser-only remote mode without weakening the
existing Content Security Policy (CSP).

## Root Cause

The LAN static handler injects an inline assignment of
`window.__FARM_GO_REMOTE__`. Its CSP uses `default-src 'self'`, which applies
to scripts and blocks that inline assignment. The frontend consequently starts
the desktop Wails application instead of `RemoteApp`, then calls unavailable
Wails browser APIs and renders blank.

## Design

The LAN handler will expose one fixed, same-origin bootstrap asset at
`/farm-go-remote.js`. It returns only:

```js
window.__FARM_GO_REMOTE__ = true;
```

For `/` and `/index.html`, the handler injects a regular external script tag
before `</head>`:

```html
<script src="/farm-go-remote.js"></script>
```

The browser executes that same-origin script before the module entrypoint in
the page body, so `main.tsx` selects `RemoteApp`. The existing CSP remains
unchanged; no inline-script exception, nonce, hash, or relaxed source list is
introduced.

The bootstrap route accepts only `GET`, returns JavaScript with `no-store`
inherited from the main handler, and is available before authentication just
like the static HTML. It contains no user data and creates no RPC capability.

## Rejected Alternatives

- Adding `'unsafe-inline'` to CSP would make the current HTML work but
  weakens the security boundary for all LAN clients.
- Using a per-response CSP nonce adds state and response rewriting without a
  need for dynamic script content.

## Validation

Server tests will first assert that the LAN index contains the external
bootstrap asset, does not contain the inline assignment, and retains the
strict CSP. A second test will verify the bootstrap route's method, content
type, and exact body. Focused Go tests, the complete frontend test suite, and
the frontend build will then run.
