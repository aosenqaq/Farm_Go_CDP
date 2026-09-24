# Disable Specified-Time Interval Input Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Disable scheduler-center interval editing for reward tasks configured with `daily_time`, while preserving and displaying the stored interval value.

**Architecture:** Keep the rule in `AutomationView.tsx` using an explicit task-ID-to-schedule-mode-key map. A pure exported helper reads the current config so rendering and tests share the same decision, and the existing interval synchronization code remains untouched.

**Tech Stack:** React 18, TypeScript, Vitest, server-side React rendering.

---

### Task 1: Disable Reward Interval Inputs

**Files:**
- Modify: `frontend/src/views/AutomationView.tsx:125-151,646-666`
- Test: `frontend/src/views/AutomationView.test.tsx:119-161`

- [ ] **Step 1: Write the failing rendering test**

Import `isSchedulerTaskIntervalDisabled` and add assertions for the pure rule plus rendered native disabled state:

```tsx
it('disables scheduler interval only for rewards using specified time', () => {
  const config = {
    ...state.config,
    autoFarmMailRewardScheduleMode: 'daily_time',
  };

  expect(isSchedulerTaskIntervalDisabled(config, 'mail_reward')).toBe(true);
  expect(isSchedulerTaskIntervalDisabled({ ...config, autoFarmMailRewardScheduleMode: 'interval' }, 'mail_reward')).toBe(false);
  expect(isSchedulerTaskIntervalDisabled(config, 'own_base')).toBe(false);

  const html = renderToStaticMarkup(
    <AutomationView
      state={{
        ...state,
        config,
        scheduler: {
          ...state.scheduler,
          tasks: state.scheduler.tasks.map((task) => task.id === 'mail_reward' ? { ...task, intervalSec: 43200 } : task),
        },
      }}
      onRunTask={() => undefined}
      initialSchedulerOpen
    />,
  );

  expect(html).toMatch(/name="interval-mail_reward"[^>]*disabled=""[^>]*value="43200"|name="interval-mail_reward"[^>]*value="43200"[^>]*disabled=""/);
  expect(html).not.toMatch(/name="interval-own_base"[^>]*disabled=""/);
});
```

- [ ] **Step 2: Run the focused suite and verify RED**

Run:

```powershell
Set-Location frontend
npm test -- --run src/views/AutomationView.test.tsx
```

Expected: build failure because `isSchedulerTaskIntervalDisabled` is not exported.

- [ ] **Step 3: Implement the minimal mapping and disabled property**

Add the eight reward schedule mode keys:

```tsx
const schedulerTaskScheduleModeConfigKeyById: Record<string, string> = {
  reward_claim: 'autoRewardClaimScheduleMode',
  svip_daily_gift: 'autoFarmSvipDailyGiftScheduleMode',
  monthly_card_reward: 'autoFarmMonthlyCardRewardScheduleMode',
  mall_daily_fertilizer: 'autoFarmMallDailyFertilizerScheduleMode',
  share_reward: 'autoFarmShareRewardScheduleMode',
  mail_reward: 'autoFarmMailRewardScheduleMode',
  he_feng_travel_reward: 'autoFarmHeFengTravelRewardScheduleMode',
  limited_seed_draw: 'autoFarmLimitedSeedDrawScheduleMode',
};

export function isSchedulerTaskIntervalDisabled(config: Record<string, any>, taskId: string) {
  const modeKey = schedulerTaskScheduleModeConfigKeyById[taskId];
  return Boolean(modeKey) && String(config?.[modeKey] || 'interval') === 'daily_time';
}
```

In the scheduler task loop, compute the rule and apply it to the interval input:

```tsx
disabled={isSchedulerTaskIntervalDisabled(schedulerState.config, task.id)}
```

Do not change `value`, `onChange`, or interval synchronization.

- [ ] **Step 4: Run focused and full frontend verification**

Run:

```powershell
Set-Location frontend
npm test -- --run src/views/AutomationView.test.tsx
npm test
npm run build
```

Expected: all commands pass; the existing 119 tests plus the new regression test pass.

- [ ] **Step 5: Commit the implementation**

```powershell
Set-Location ..
git add frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx docs/superpowers/plans/2026-07-10-disable-specified-time-interval-input.md
git diff --cached --check
git commit -m "fix: disable interval for scheduled rewards"
```
