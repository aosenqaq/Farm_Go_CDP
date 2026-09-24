# License UI Polish Design

## Scope

Refine the existing KAuth-facing user experience in three connected surfaces:

1. Localize the license gate fully into Chinese.
2. Present detected updates in one reusable modal instead of navigating users away to Settings.
3. Replace the overview's placeholder authorization status with a compact authorization service card.

## License Gate

Keep the current centered warm-white gate, password behavior, remember-card option, and status states. Replace all user-facing English license text with concise Chinese language. Backend error codes remain internal; the UI renders only its existing safe status message or a Chinese fallback.

## Update Modal

Use one modal controlled by the authorized application root. Both the update toast action and a successful manual update check request the same modal. The modal displays current version, latest version, release description, and a download command only when the backend-provided URL is present. The browser is opened only from that command; opening the modal never opens a URL.

The update toast remains non-blocking. A manual check with no update or an error remains represented in the Settings panel without forcing the modal.

## Authorization Service Card

Replace the fifth overview strip card with the approved compact A treatment:

- Sparkle icon and bold black title `授权服务`.
- Green rounded status band for the authorization state and heartbeat health.
- Secondary expiration row below the band.
- Use `—` when expiration time is absent or unavailable.

The card receives only safe license status from the authorized application root. It must not expose card values, tokens, or raw KAuth responses.

## Data Flow

`AppBootstrap` owns normalized license status. It passes safe status to `AuthorizedApp`, which passes it to the overview through its existing view composition. The authorized root also owns update-modal visibility and the selected update state. Settings and the toast call callbacks rather than storing independent modal state.

## Verification

- Focused component tests cover Chinese license fallbacks, modal open sources and safe download command, and authorization card state/expiry placeholders.
- Run the frontend test suite and production build after implementation.
