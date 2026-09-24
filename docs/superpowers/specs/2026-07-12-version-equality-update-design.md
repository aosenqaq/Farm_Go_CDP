# Version Equality Update Check Design

## Goal

Do not report an update when the local and remote user-visible version labels describe the same release.

## Rule

The update comparison normalizes both version labels by trimming whitespace, removing a leading `v` or `V`, and comparing without case sensitivity. If both normalized labels are non-empty and equal, `Available` is false even when KAuth's internal version numbers differ.

If either label is empty or the normalized labels differ, the existing numeric comparison remains the source of truth.

## User Experience

The backend returns `available: false` for equivalent labels such as `1.0.0` and `v1.0.0`. Existing frontend behavior then suppresses the update toast and detail dialog. After a manual check, the settings page renders `已是最新版本`.

## Verification

Add a backend regression test for an internal remote number greater than the local one with equivalent display labels. The test must assert that the update is unavailable. Run the focused license tests, full Go tests, frontend tests, and the frontend build.
