# Auto Farm Scheduler Migration Design

## Goal

Migrate the reference project's `自动农场` and `调度中心` blocks into Farm_Go with the approved A-style desktop tool UI: existing Farm_Go visual language, large feature cards, per-feature master switches, settings icons, and a top-right scheduler dialog.

## Reference Sources

- Reference project: `E:\desktop\farm-tauri-core-copy-20260707-155223\farm-tauri`
- Automation state and scheduling: `core/src/auto-farm-manager.js`
- Automation execution behavior: `core/src/auto-farm-executor.js`
- Gateway API shape: `core/src/gateway.js`
- Recommended config: `core/src/recommended-farm-config.js`
- Existing Farm_Go UI shell: `frontend/src/components/AppShell.tsx`
- Current placeholder view: `frontend/src/views/FarmWorkspaceView.tsx`

## Approved UI Direction

Use visual option A as the base:

- Keep Farm_Go's existing dark green sidebar, warm light surface, compact cards, and lime primary actions.
- The `农场自动化` page gets a top header with status actions and a right-side `调度中心` button.
- Large automation features render as compact cards with:
  - feature icon or short glyph,
  - title and one-line summary,
  - master ON/OFF switch,
  - settings icon button.
- Clicking a settings icon opens a focused settings dialog for that feature group.
- Clicking `调度中心` opens a modal page from the top-right area. It controls task priority, interval display, scheduler min gap, and manual task execution.

## Feature Grouping

The reference automation settings should be presented as seven large feature groups:

1. `自家基础任务`
   Covers one-click farm tasks, auto harvest, grass removal, watering, bug killing, dead crop cleanup, and "only collect when own farm".

2. `自动种植策略`
   Covers primary and secondary plant strategies, specified seed, backpack seed priority, disabled backpack seeds, force priority, four-grid crops, random planting delay, random land order, and max plant level.

3. `肥料与催熟`
   Covers auto fertilizer, fertilizer fill, plant fertilizer mode, rush fertilizer mode, multi-season fertilizer, harvest-link rush, continuous rush, fertilizer auto-buy, fertilizer land types, rush threshold, max lands per run, and call timeouts.

4. `好友自动化`
   Covers friend steal, friend help, friend mischief, per-round friend count, daily limits, visit cooldown, blacklist-only cooldown, quiet hours, friend whitelist/blacklist, masked-friend blacklist, and steal crop black/white lists.

5. `奖励与活动`
   Covers task reward claim, SVIP daily gift, monthly card reward, mall daily fertilizer, share reward, mail reward, He Feng travel reward, and limited seed draw.

6. `神秘商店自动购买`
   Covers mystery shop auto-buy, target seeds, currency types, discount threshold, interval, and once-per-day processed state.

7. `运行节奏与附属联动`
   Covers auto-start, enter wait, action wait, RPC timeout, scheduler min gap, reward popup interception, warehouse auto refresh, warehouse refresh-only-on-sell, auto warehouse sell, sell interval, and sell categories.

## Scheduler Tasks

The first migration keeps the reference task IDs and labels:

| Task ID | Label | Default Priority | Interval Field |
| --- | --- | ---: | --- |
| `own_base` | 一键务农 | 100 | `autoFarmOwnBaseIntervalSec` |
| `land_upgrade` | 土地自动升级 | 99 | `autoFarmLandUpgradeIntervalSec` |
| `reward_claim` | 自动领取任务奖励 | 98 | `autoRewardClaimIntervalSec` |
| `svip_daily_gift` | SVIP每日礼包 | 97 | `autoFarmSvipDailyGiftIntervalSec` |
| `monthly_card_reward` | 月卡奖励 | 96 | `autoFarmMonthlyCardRewardIntervalSec` |
| `mall_daily_fertilizer` | 商城每日肥料 | 95 | `autoFarmMallDailyFertilizerIntervalSec` |
| `share_reward` | 自动领取分享奖励 | 94 | `autoFarmShareRewardIntervalSec` |
| `mail_reward` | 自动领取邮件奖励 | 93 | `autoFarmMailRewardIntervalSec` |
| `he_feng_travel_reward` | 限时活动/荷风游记奖励领取 | 92 | `autoFarmHeFengTravelRewardIntervalSec` |
| `limited_seed_draw` | 限时活动/荷风游记抽奖 | 92 | `autoFarmLimitedSeedDrawIntervalSec` |
| `mystery_shop_auto_buy` | 神秘商店自动购买 | 91 | `autoFarmMysteryShopAutoBuyIntervalSec` |
| `own_collect` | 自动收获 | 91 | `autoFarmOwnCollectIntervalSec` |
| `own_plant` | 自动种植 | 90 | `autoFarmPlantIntervalSec` |
| `fertilizer_fill` | 自动填充化肥 | 86 | `autoFarmFertilizerFillIntervalSec` |
| `own_fertilizer` | 自动施肥 | 85 | `autoFarmFertilizerIntervalSec` |
| `friend_steal` | 好友偷菜 | 70 | `autoFarmFriendStealIntervalSec` |
| `friend_help` | 好友帮忙 | 65 | `autoFarmFriendHelpIntervalSec` |
| `friend_mischief` | 好友捣乱 | 60 | `autoFarmFriendMischiefIntervalSec` |

## Migration Phasing

### Phase 1: Configuration and UI Shell

Build the Go-native automation catalog, normalized config defaults, scheduler snapshot, Wails methods, and React A-style page. Manual execution buttons return structured `not_migrated` until runtime execution is wired.

### Phase 2: Runtime Command Facade

Introduce a farm automation runtime facade that can call live QQ/WeChat runtime paths through existing Farm_Go runtime links. Each task gets a narrow command surface and structured errors.

### Phase 3: Feature-by-Feature Script Migration

Migrate executor behavior one task family at a time: own farm, planting, fertilizer, friend tasks, rewards, mystery shop, warehouse links. Each family gets tests, UI settings, runtime error handling, and a local commit.

## Data Flow

React calls Wails bindings on `App`.

`App` delegates to a focused Go service under `internal/farm/automation`.

The service returns:

- automation config defaults,
- seven feature group cards,
- scheduler tasks sorted by priority,
- structured action results.

When runtime execution is not implemented, action results must use:

```json
{ "ok": false, "status": "not_migrated", "message": "..." }
```

No UI control may claim a task ran successfully until the real runtime path is migrated and verified.

## Error Handling

All automation actions should return one of:

- `ok`
- `not_migrated`
- `runtime_not_ready`
- `unsupported_target`
- `busy`
- `failed`

The UI should keep the page usable when runtime actions are unavailable: switches and config editing remain visible, but manual run buttons show the structured status text.

## Testing

Phase 1 requires:

- Go tests for default config normalization, feature group coverage, scheduler task ordering, and `not_migrated` action results.
- Frontend tests for rendering the seven feature groups, top-right scheduler entry, scheduler rows, and manual action callback shape.
- Full verification with `go test ./...`, `cd frontend && npm test`, and `cd frontend && npm run build`.

## Commit Policy

Use separate commits for:

- design and plan docs,
- backend automation model,
- frontend automation UI,
- runtime execution slices added later.
