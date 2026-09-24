# LAN Mobile Friend Action Labels Design

## Goal

Restore visible, understandable friend-row action controls in the LAN WebUI on phone widths. The controls remain compact and retain their existing action and configuration menus.

## Root Cause

The LAN service serves the frontend assets embedded in the running Farm Go executable. The screenshot is from an older embedded bundle whose friend-row controls contain only SVG icons. The current remote-phone rule hides the SVGs to reduce visual noise, leaving empty touch targets. Current frontend source already renders the intended `操作` and `配置` text labels, but an updated build must be embedded in the executable serving the LAN page.

## Approved Design

At widths up to 760px within `.app-shell-remote`, each friend row retains a 114px right-side action region with two adjacent controls:

- `操作` and `配置` are explicit text labels, with no visible chevron on phones.
- Each visual button is 55px wide and 32px high; the native button keeps a 42px high touch target.
- The `操作` button continues to open the existing command sheet for view, steal, mischief, and help.
- The `配置` button continues to open the existing command sheet for local black/white list and system blacklist management.

No friend action, rule, API, desktop layout, or LAN authentication behavior changes.

## Implementation

The frontend source and its focused tests already define the approved text-button structure and remote-phone dimensions. The implementation refreshes the generated Vite bundle, then rebuilds the application binary so the LAN server's embedded `frontend/dist` files match the source. No source behavior changes are expected unless verification reveals an asset mismatch in the current build output.

## Verification

1. Run the focused social view test and the complete frontend test suite.
2. Build the frontend and confirm the generated CSS and JavaScript contain the labeled friend controls.
3. Build the application so the embedded asset filesystem is refreshed.
4. At a 360px LAN viewport, verify both labels are readable, each target is touchable, and their command sheets still open.
5. Verify a desktop viewport remains unchanged.

## Non-Goals

- Redesigning friend actions or their command sheets.
- Changing local-list or system-blacklist behavior.
- Altering remote desktop layout or network access settings.
