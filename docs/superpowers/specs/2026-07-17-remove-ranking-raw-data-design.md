# Remove Ranking Raw Data Design

## Goal

Remove the ranking dialog's "原始数据" button and the unreachable raw JSON viewing mode it controls.

## Design

The ranking dialog will always render its normal list or ranking presentation. The existing ranking preference controls remain available on steal-record tabs, while the raw-mode React state, button, component property, JSON rendering branch, and dedicated CSS are removed.

## Verification

Update the ranking dialog render test to assert that "原始数据" is absent, remove the obsolete raw-mode interaction test, then run the SocialView tests and the frontend production build.
