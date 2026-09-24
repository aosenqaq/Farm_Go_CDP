# Minimal Workbench Design

## Goal

Replace the card-matrix runtime overview with a sparse operational view that answers one question first: whether the current runtime is ready to use.

## Composition

The page begins with the existing page title and refresh control. Its primary content is a single runtime hero showing the selected target and the ready state. A compact readiness label provides the action-level conclusion.

Below the hero, a single inline signal row presents gateway, process guard, authorization state, and authorization expiry. Signals are text with small status dots, not cards. Authorization remains a safe display projection with no card, token, or raw service message.

The lower section has two unframed columns: recent events as the primary area and current route facts as a narrow secondary column. At small widths, these stack without horizontal scrolling.

## Data And States

The existing runtime, guard, safe authorization, event, and refresh inputs remain unchanged. The hero state is derived from the current runtime status. Service signal colors preserve existing healthy, warning, and unavailable semantics. Missing authorization expiry renders `—`.

## Interaction

The existing refresh command remains the only header action. No additional navigation or automation controls are added. The event list continues its existing automatic refresh behavior.

## Verification

- Update overview tests for hero state, inline authorization signal, and compact route facts.
- Verify the narrow-screen stacking rule without relying on brittle formatting assertions.
- Run the frontend test suite and production build.
