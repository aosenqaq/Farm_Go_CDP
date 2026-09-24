# Previous-Day Daily Report Design

**Status:** Approved for implementation planning

## Goal

Make every daily report, including scheduled delivery and the manual "send daily
report now" action, describe the complete previous Beijing calendar day. A report
sent at 2026-07-21 09:00 therefore identifies 2026-07-20 and excludes all
2026-07-21 activity.

## Scope

- Apply one previous-day date key to `daily.date`, daily payload metadata, sales,
  and automation activity values.
- Use persisted runtime events and warehouse sale records for that exact date.
- Update built-in wording so it does not call historical activity "today".
- Preserve the existing once-per-delivery-day scheduler deduplication behavior.

The change does not add date selection, historical report backfill, or changes to
user-authored templates.

## Data Flow

```
scheduled or manual daily send at T
  -> messagepush derives Beijing date T - 1 day
  -> App loads activity and sales matching that date
  -> template context exposes matching daily.date and values
  -> existing channel delivery and delivery-day deduplication
```

The message-push service owns the default report date. The app-level report provider
uses the same target date when aggregating account activity. It must not fall back to
the live profile's `TodayStats`, because those values describe the delivery day rather
than the report day.

## Components

`internal/messagepush` will default `daily.date` and the daily payload date metadata
to `ShiftDateKey(now, -1)`. The daily-report provider result can carry the exact date
it aggregated so the rendered context remains coupled to the data source.

`App.dailyReportForAccount` will select the prior date from a two-day activity window,
sum warehouse sales whose stored `date_key` matches it, and return zero-valued activity
when the persisted history contains no events for that date. It will retain live
profile and warehouse reads for account identity and asset snapshots.

Built-in daily summary/template text will refer to the report period as "当日" rather
than "今日". Existing custom templates retain their content and can use `daily.date`.

## Scheduling and Errors

`LastDailySummaryDateKey` continues to record the Beijing delivery date. This avoids
skipping the first report after upgrade when older state was written using the prior
meaning, while preserving the existing one-send-per-scheduled-day rule.

Existing runtime, warehouse, store, and delivery errors remain unchanged. If no prior
day activity is available, the report is sent with zero activity rather than incorrect
current-day activity.

## Verification

- Add an application-level regression test with distinct previous-day and current-day
  automation events and sale records; only previous-day totals may appear in the
  report.
- Add a message-push test showing a manual daily send at a fixed time renders the
  previous Beijing date.
- Keep the existing due-scheduler idempotency test to prove deduplication still uses
  the delivery day.
- Run targeted message-push and root application tests, then the complete Go suite.
