import type { Tab } from '../components/AppShell';
import type { RuntimeEventDto } from '../lib/events';
import type { RankingDataVersions, RankingPage, RankingPageRequest } from '../lib/socialRankingPages';
import { AssetsLandView } from './AssetsLandView';
import { AutomationView, type FarmAutomationActionResult, type FarmAutomationState, type MysteryShopPurchaseRecord } from './AutomationView';
import type { GuardStatusDto, HostBindingStatusDto } from './GuardView';
import { OverviewView, type RuntimeStatusDto, type WorkspaceRunStatistics } from './OverviewView';
import {
  SocialView,
  type DogGuardState,
  type ImportExportPayload,
  type RankingPreferences,
  type SaveTextFileRequest,
  type SaveTextFileResult,
  type SocialActionRequest,
  type SocialActionResult,
  type SocialState,
} from './SocialView';

type WorkspaceArea = Extract<Tab, 'workspace' | 'automation' | 'assets' | 'social'>;

type FarmWorkspaceViewProps = {
  area: WorkspaceArea;
  remote?: boolean;
  status: RuntimeStatusDto;
  guardStatus: GuardStatusDto;
  bindingStatus?: HostBindingStatusDto | null;
  events: RuntimeEventDto[];
  tsdkEvents?: RuntimeEventDto[];
  runStatistics?: WorkspaceRunStatistics | null;
  onRefresh: () => void;
  onLaunch: () => void;
  onRestart: () => void;
  onToggleGuard: (enabled: boolean) => void;
  automationState?: FarmAutomationState | null;
  automationActionResult?: FarmAutomationActionResult | null;
  mysteryShopPurchaseRecords?: MysteryShopPurchaseRecord[];
  onAutomationActionResultConsumed?: () => void;
  onRunAutomationTask?: (taskId: string) => void | Promise<void>;
  onToggleAutomation?: (running: boolean) => void | Promise<void>;
  onSaveAutomationState?: (state: FarmAutomationState) => void | Promise<FarmAutomationState | void>;
  onSetAutomationRunMode?: (runMode: 'safe' | 'god') => Promise<FarmAutomationState | void>;
  onRefreshBackpackSeeds?: (config: Record<string, any>) => Promise<any>;
  onRefreshStealCropOptions?: () => Promise<any>;
  socialState?: SocialState | null;
  socialRankingPreferences?: RankingPreferences | null;
  socialRankingDataVersions?: RankingDataVersions;
  socialDogGuardState?: DogGuardState | null;
  socialProtocolBlockList?: SocialState['friends'];
  socialActionResult?: SocialActionResult | null;
  onSocialActionResultConsumed?: () => void;
  onRefreshSocial?: (refresh?: boolean) => void | Promise<void>;
  onSocialAction?: (request: SocialActionRequest) => void | Promise<SocialActionResult | void>;
  onSocialRankings?: (request: RankingPageRequest) => Promise<RankingPage>;
  onRefreshSocialVisitors?: () => Promise<SocialActionResult>;
  onSaveSocialRankingPreferences?: (preferences: RankingPreferences) => void | Promise<RankingPreferences | void>;
  onSocialDogGuardAction?: (request: { action: string; refresh?: boolean; scanIntervalMs?: number; skipScanned?: boolean; excludeGuardDog?: boolean }) => void | Promise<DogGuardState | void>;
  onSocialProtocolBlockList?: () => void | Promise<void>;
  onSocialExport?: (request: { groups: string[] }) => Promise<ImportExportPayload | void> | void;
  onSocialImport?: (payload: ImportExportPayload) => Promise<SocialActionResult | void> | void;
  onSaveTextFile?: (request: SaveTextFileRequest) => Promise<SaveTextFileResult | void> | void;
  toggling?: boolean;
};

const automationState: FarmAutomationState = {
  running: false,
  runMode: 'safe',
  summary: { enabledTasks: 8, totalTasks: 18, todayHarvest: 0 },
  config: {},
  featureGroups: [
    { id: 'own_base', label: '基础任务', summary: '一键务农、自动收获、除草、浇水、杀虫。', enabled: true, settingKeys: ['autoFarmOneClickEnabled', 'autoFarmOwnCollectEnabled'] },
    { id: 'planting', label: '自动种植', summary: '主/副策略、背包种子选择、随机延迟。', enabled: true, settingKeys: ['autoFarmPlantPrimaryMode', 'autoFarmPlantSecondaryMode'] },
    { id: 'fertilizer', label: '自动施肥', summary: '自动施肥、催熟联动、自动购买。', enabled: false, settingKeys: ['autoFarmFertilizerEnabled'] },
    { id: 'friends', label: '好友互动', summary: '偷菜、帮忙、捣乱、冷却与静默时间。', enabled: false, settingKeys: ['autoFarmFriendEnabled', 'autoFarmFriendHelpEnabled'] },
    { id: 'rewards', label: '自动领取奖励', summary: '任务奖励、礼包、月卡、邮件、抽奖。', enabled: true, settingKeys: ['autoRewardClaimEnabled', 'autoFarmSvipDailyGiftEnabled'] },
    { id: 'mystery_shop', label: '神秘商店自动购买', summary: '货币类型、折扣阈值和手动读取购买。', enabled: false, settingKeys: ['autoFarmMysteryShopAutoBuyEnabled'] },
  ],
  scheduler: {
    enabled: true,
    minGapMs: 350,
    tasks: [
      { id: 'own_base', label: '一键务农', priority: 100, intervalSec: 60, enabled: true },
      { id: 'land_upgrade', label: '土地自动升级', priority: 99, intervalSec: 43200, enabled: true },
      { id: 'reward_claim', label: '自动领取任务奖励', priority: 98, intervalSec: 3600, enabled: true },
      { id: 'svip_daily_gift', label: 'SVIP每日礼包', priority: 97, intervalSec: 43200, enabled: true },
      { id: 'monthly_card_reward', label: '月卡奖励', priority: 96, intervalSec: 43200, enabled: true },
      { id: 'mall_daily_fertilizer', label: '商城每日肥料', priority: 95, intervalSec: 43200, enabled: true },
      { id: 'share_reward', label: '自动领取分享奖励', priority: 94, intervalSec: 43200, enabled: true },
      { id: 'mail_reward', label: '自动领取邮件奖励', priority: 93, intervalSec: 43200, enabled: true },
      { id: 'qian_xing_travel_reward', label: '千星游记奖励领取', priority: 92, intervalSec: 7200, enabled: false },
      { id: 'xing_su_auto_light_up', label: '自动点亮星宿', priority: 91, intervalSec: 7200, enabled: false },
      { id: 'limited_seed_draw', label: '荷风游记抽奖', priority: 92, intervalSec: 43200, enabled: false },
      { id: 'mystery_shop_auto_buy', label: '神秘商店自动购买', priority: 91, intervalSec: 43200, enabled: false },
      { id: 'own_collect', label: '自动收获', priority: 91, intervalSec: 30, enabled: true },
      { id: 'own_plant', label: '自动种植', priority: 90, intervalSec: 10, enabled: true },
      { id: 'fertilizer_fill', label: '自动填充化肥', priority: 86, intervalSec: 43200, enabled: false },
      { id: 'own_fertilizer', label: '自动施肥', priority: 85, intervalSec: 30, enabled: false },
      { id: 'friend_steal', label: '好友偷菜', priority: 70, intervalSec: 90, enabled: false },
      { id: 'friend_help', label: '好友帮忙', priority: 65, intervalSec: 90, enabled: false },
      { id: 'friend_mischief', label: '好友捣乱', priority: 60, intervalSec: 90, enabled: false },
    ],
  },
};

export function FarmWorkspaceView({
  area,
  remote = false,
  status,
  guardStatus,
  bindingStatus,
  events,
  tsdkEvents = [],
  runStatistics,
  onRefresh,
  onLaunch,
  onRestart,
  onToggleGuard,
  automationState: externalAutomationState,
  automationActionResult,
  mysteryShopPurchaseRecords,
  onAutomationActionResultConsumed,
  onRunAutomationTask,
  onToggleAutomation,
  onSaveAutomationState,
  onSetAutomationRunMode,
  onRefreshBackpackSeeds,
  onRefreshStealCropOptions,
  socialState,
  socialRankingPreferences,
  socialRankingDataVersions,
  socialDogGuardState,
  socialProtocolBlockList,
  socialActionResult,
  onSocialActionResultConsumed,
  onRefreshSocial,
  onSocialAction,
  onSocialRankings,
  onRefreshSocialVisitors,
  onSaveSocialRankingPreferences,
  onSocialDogGuardAction,
  onSocialProtocolBlockList,
  onSocialExport,
  onSocialImport,
  onSaveTextFile,
  toggling = false,
}: FarmWorkspaceViewProps) {
  if (area === 'workspace') {
    return <OverviewView status={status} guardStatus={guardStatus} events={events} tsdkEvents={tsdkEvents} runStatistics={runStatistics} onRefresh={onRefresh} />;
  }

  if (area === 'assets') {
    return <AssetsLandView />;
  }

  if (area === 'automation') {
    return (
      <AutomationView
        state={externalAutomationState || automationState}
        remote={remote}
        dogGuardFriendCount={socialState?.summary.dogGuardCount || 0}
        onRunTask={onRunAutomationTask || (() => undefined)}
        onToggleAutomation={onToggleAutomation}
        onSaveState={onSaveAutomationState}
        onSetRunMode={onSetAutomationRunMode}
        onRefreshBackpackSeeds={onRefreshBackpackSeeds}
        onRefreshStealCropOptions={onRefreshStealCropOptions}
        lastActionResult={automationActionResult}
        onActionResultConsumed={onAutomationActionResultConsumed}
        mysteryShopPurchaseRecords={mysteryShopPurchaseRecords}
      />
    );
  }

  if (area === 'social') {
    return (
      <SocialView
        state={socialState || null}
        isQQRuntime={status.target === 'qq_ws'}
        rankingPreferences={socialRankingPreferences || undefined}
        rankingDataVersions={socialRankingDataVersions}
        dogGuardState={socialDogGuardState || null}
        protocolBlockList={socialProtocolBlockList || []}
        lastActionResult={socialActionResult || null}
        onActionResultConsumed={onSocialActionResultConsumed}
        onRefresh={onRefreshSocial || (() => undefined)}
        onAction={onSocialAction || (() => undefined)}
        onRankings={onSocialRankings}
        onRefreshVisitors={onRefreshSocialVisitors}
        onSaveRankingPreferences={onSaveSocialRankingPreferences}
        onDogGuardAction={onSocialDogGuardAction}
        onProtocolBlockList={onSocialProtocolBlockList}
        onExport={onSocialExport}
        onImport={onSocialImport}
        onSaveTextFile={remote ? undefined : onSaveTextFile}
      />
    );
  }

  return null;
}
