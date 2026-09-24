import {
  Ban,
  Bug,
  Check,
  ChevronDown,
  Download,
  Eye,
  HeartHandshake,
  ListFilter,
  RefreshCcw,
  Shield,
  ShieldCheck,
  Sparkles,
  Swords,
  Trophy,
  Unlock,
  Upload,
  Users,
  X,
} from 'lucide-react';
import type { MouseEvent, ReactNode } from 'react';
import { useEffect, useMemo, useRef, useState } from 'react';

import type { RankingDataVersions, RankingPage, RankingPageRequest } from '../lib/socialRankingPages';
import { SocialRankingDialog, type RankingPreferences } from './social/SocialRankingDialog';

export type { RankingPreferences } from './social/SocialRankingDialog';

export type SocialStatus = 'ok' | 'runtime_not_ready' | 'unsupported_target' | 'busy' | 'skipped' | 'failed' | string;

export const SOCIAL_TOAST_AUTO_DISMISS_MS = 3500;

export type FriendRules = {
  whitelistEnabled: boolean;
  whitelistScopes: string[];
  whitelist: string[];
  blacklistEnabled: boolean;
  blacklistScopes: string[];
  blacklist: string[];
  maskedBlacklist: boolean;
  maskedMaxLevel: number;
};

export type FriendRow = {
  gid: number;
  name?: string;
  displayName: string;
  remark?: string;
  avatarUrl?: string;
  level?: number;
  workCounts: Record<string, number>;
  stealable: boolean;
  helpable: boolean;
  mischiefable: boolean;
  blacklisted: boolean;
  whitelisted: boolean;
  maskedBlocked: boolean;
  protected: boolean;
  protocolBlocked: boolean;
  hasGuardDog: boolean;
  raw?: Record<string, unknown>;
};

export type FriendSortMode = 'default' | 'level_asc' | 'level_desc';

export function sortFriendsByLevel(friends: FriendRow[], sortMode: FriendSortMode): FriendRow[] {
  if (sortMode === 'default') return friends;
  const direction = sortMode === 'level_asc' ? 1 : -1;
  return friends
    .map((friend, index) => ({ friend, index, level: Number.isFinite(friend.level) ? Number(friend.level) : null }))
    .sort((left, right) => {
      if (left.level === null && right.level === null) return left.index - right.index;
      if (left.level === null) return 1;
      if (right.level === null) return -1;
      return (left.level - right.level) * direction || left.index - right.index;
    })
    .map(({ friend }) => friend);
}

export type SocialSummary = {
  totalFriends: number;
  stealableFriends: number;
  helpableFriends: number;
  mischiefFriends: number;
  blacklisted: number;
  whitelisted: number;
  maskedBlocked: number;
  protected: number;
  dogGuardCount: number;
};

export type SocialState = {
  ok: boolean;
  status: SocialStatus;
  message: string;
  summary: SocialSummary;
  rules: FriendRules;
  friends: FriendRow[];
};

export type SocialActionRequest = {
  action: string;
  target?: string;
  targets?: string[];
  rules?: FriendRules;
  dryRun?: boolean;
};

export type SocialActionResult = {
  ok: boolean;
  status: SocialStatus;
  message: string;
  data?: unknown;
};

type GodRankRow = {
  gid?: number;
  displayName?: string;
  name?: string;
  remark?: string;
  isBanned?: boolean;
  is_banned?: boolean;
  displayRank?: number;
  rank?: number;
  level?: number;
  stealableCount?: number;
  helpableCount?: number;
  score?: number;
};

type GodRankPayload = {
  count?: number;
  list: GodRankRow[];
};

export type StealRecord = {
  id: string;
  gid: number;
  displayName: string;
  stealCount: number;
  action: string;
  occurredAt: string;
  landIds?: number[];
  items?: Array<{
    itemId?: number;
    name: string;
    count: number;
    landIds?: number[];
  }>;
};

export type VisitorRecord = {
  id: string;
  playerId: number;
  displayName: string;
  actionType: number;
  time: number;
  stealItemId?: number;
  stealItemName?: string;
  stealItemNum?: number;
  actionLabel: string;
};

export type DogGuardRow = {
  gid: number;
  name: string;
  displayName?: string;
  level?: number;
  scanned: boolean;
  hasGuardDog: boolean;
  dogId?: number;
  dogName?: string;
  error?: string;
  scannedAt?: string;
};

export type DogGuardState = {
  running: boolean;
  stopRequested: boolean;
  startedAt?: string;
  finishedAt?: string;
  total: number;
  scanned: number;
  hasGuardDogCount: number;
  current?: DogGuardRow;
  results: DogGuardRow[];
  error?: string;
};

export type ImportExportPayload = {
  version: number;
  groups: string[];
  accountKey?: string;
  rules?: FriendRules;
  friendSnapshot?: FriendRow[];
  stealRecords?: StealRecord[];
  visitorRecords?: VisitorRecord[];
  dogGuard?: DogGuardState;
};

export type SaveTextFileRequest = {
  defaultName: string;
  content: string;
  filters?: { name: string; extensions: string[] }[];
};

export type SaveTextFileResult = {
  path: string;
};

type SocialViewProps = {
  state: SocialState | null;
  isQQRuntime?: boolean;
  rankingPreferences?: RankingPreferences;
  rankingDataVersions?: RankingDataVersions;
  dogGuardState?: DogGuardState | null;
  protocolBlockList?: FriendRow[];
  lastActionResult?: SocialActionResult | null;
  onActionResultConsumed?: () => void;
  onRefresh: (refresh?: boolean) => void | Promise<void>;
  onAction: (request: SocialActionRequest) => void | Promise<SocialActionResult | void>;
  onRankings?: (request: RankingPageRequest) => Promise<RankingPage>;
  onRefreshVisitors?: () => Promise<SocialActionResult>;
  onSaveRankingPreferences?: (preferences: RankingPreferences) => void | Promise<RankingPreferences | void>;
  onDogGuardAction?: (request: {
    action: string;
    refresh?: boolean;
    scanIntervalMs?: number;
    skipScanned?: boolean;
    excludeGuardDog?: boolean;
  }) => void | Promise<DogGuardState | void>;
  onProtocolBlockList?: () => void | Promise<void>;
  onExport?: (request: { groups: string[] }) => Promise<ImportExportPayload | void> | void;
  onImport?: (payload: ImportExportPayload) => Promise<SocialActionResult | void> | void;
  onSaveTextFile?: (request: SaveTextFileRequest) => Promise<SaveTextFileResult | void> | void;
  initialRankingOpen?: boolean;
  initialFeatureOpen?: boolean;
  initialRulesOpen?: boolean;
  initialDogOpen?: boolean;
  initialImportExportOpen?: boolean;
  initialProtocolOpen?: boolean;
};

const emptyRules: FriendRules = {
  whitelistEnabled: false,
  whitelistScopes: [],
  whitelist: [],
  blacklistEnabled: false,
  blacklistScopes: [],
  blacklist: [],
  maskedBlacklist: true,
  maskedMaxLevel: 1,
};

const emptyState: SocialState = {
  ok: false,
  status: 'runtime_not_ready',
  message: '',
  summary: {
    totalFriends: 0,
    stealableFriends: 0,
    helpableFriends: 0,
    mischiefFriends: 0,
    blacklisted: 0,
    whitelisted: 0,
    maskedBlocked: 0,
    protected: 0,
    dogGuardCount: 0,
  },
  rules: emptyRules,
  friends: [],
};

const defaultRankingPreferences: RankingPreferences = {
  stolenByMeViewMode: 'timeline',
  stolenFromMeViewMode: 'timeline',
};

const defaultRankingDataVersions: RankingDataVersions = { stolenByMe: 0, visitors: 0 };

async function emptyRankingPage(request: RankingPageRequest): Promise<RankingPage> {
  return {
    ok: true,
    status: 'ok',
    message: '',
    tab: request.tab,
    viewMode: request.viewMode,
    dateRange: request.dateRange,
    summary: { visitorCount: 0, stolenFromMeCount: 0, stolenByMeCount: 0, stolenByMeRecordCount: 0 },
    rows: [],
    hasMore: false,
  };
}

async function unavailableVisitorRefresh(): Promise<SocialActionResult> {
  return { ok: false, status: 'failed', message: '访客刷新不可用。' };
}

async function keepRankingPreferences(preferences: RankingPreferences): Promise<RankingPreferences> {
  return preferences;
}

const emptyDogGuard: DogGuardState = {
  running: false,
  stopRequested: false,
  total: 0,
  scanned: 0,
  hasGuardDogCount: 0,
  results: [],
};

function objectFromUnknown(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function numberFromUnknown(value: unknown): number | undefined {
  const num = Number(value);
  return Number.isFinite(num) ? num : undefined;
}

function godRankPayloadFromAction(result: SocialActionResult | null | undefined): GodRankPayload | null {
  if (!result?.ok) return null;
  const data = objectFromUnknown(result.data);
  if (!data || data.action !== 'get_god_rank_list' || !Array.isArray(data.list)) return null;
  const list = data.list.map((item) => {
    const row = objectFromUnknown(item) || {};
    return {
      gid: numberFromUnknown(row.gid),
      displayName: typeof row.displayName === 'string' ? row.displayName : undefined,
      name: typeof row.name === 'string' ? row.name : undefined,
      remark: typeof row.remark === 'string' ? row.remark : undefined,
      isBanned: row.isBanned === true || row.is_banned === true,
      is_banned: row.isBanned === true || row.is_banned === true,
      displayRank: numberFromUnknown(row.displayRank),
      rank: numberFromUnknown(row.rank),
      level: numberFromUnknown(row.level),
      stealableCount: numberFromUnknown(row.stealableCount),
      helpableCount: numberFromUnknown(row.helpableCount),
      score: numberFromUnknown(row.score),
    };
  }).filter((row) => row.isBanned === true);
  return {
    count: list.length,
    list,
  };
}

const scopeOptions = [
  ['steal', '偷菜'],
  ['help', '帮忙'],
  ['mischief', '捣乱'],
] as const;

function listToText(items: string[]): string {
  return (items || []).join('\n');
}

function textToList(value: string): string[] {
  const seen = new Set<string>();
  return value
    .split(/[\r\n,，;；、|]+/)
    .map((item) => item.trim())
    .filter((item) => {
      if (!item || seen.has(item)) return false;
      seen.add(item);
      return true;
    });
}

type RuleListName = 'blacklist' | 'whitelist';
type RuleExportName = RuleListName | 'all';

function listFromUnknown(value: unknown): string[] {
  if (Array.isArray(value)) return textToList(value.map((item) => String(item ?? '')).join('\n'));
  return textToList(String(value ?? ''));
}

export function formatRuleExportList(items: string[]): string {
  return textToList((items || []).join('\n')).join('\n');
}

export function buildRuleExportFileRequest(listName: RuleExportName, items: string[]): SaveTextFileRequest {
  const defaultName = listName === 'blacklist'
    ? 'farm-blacklist.txt'
    : listName === 'whitelist'
      ? 'farm-whitelist.txt'
      : 'farm-all-friends.txt';
  return {
    defaultName,
    content: formatRuleExportList(items),
    filters: [{ name: 'Text Files', extensions: ['txt'] }],
  };
}

export function parseRuleImportText(value: string, listName: RuleListName): string[] {
  const raw = value.trim();
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    const rules = objectFromUnknown(parsed.rules);
    const rootList = objectFromUnknown(parsed);
    const candidate = rules?.[listName] ?? rootList?.[listName];
    if (candidate != null) return listFromUnknown(candidate);
  } catch {
    // Plain gid lists are the normal import path; invalid JSON falls through to text parsing.
  }
  return textToList(raw);
}

export function buildRulesWithImportedList(rules: FriendRules, listName: RuleListName, value: string): FriendRules {
  return { ...rules, [listName]: parseRuleImportText(value, listName) };
}

function normalizeRuleText(value: unknown): string {
  return String(value ?? '').trim().toLowerCase();
}

function friendMatchesRuleList(items: string[], friend: FriendRow): boolean {
  const rules = (items || []).map(normalizeRuleText).filter(Boolean);
  if (rules.length === 0) return false;
  const gid = String(friend.gid);
  const fields = [friend.displayName, friend.name, friend.remark, gid].map(normalizeRuleText).filter(Boolean);
  return rules.some((rule) => {
    if (/^\d+$/.test(rule)) return rule === gid;
    return fields.some((field) => field === rule || field.includes(rule));
  });
}

function isFriendBlacklistedByRules(friend: FriendRow, rules: FriendRules): boolean {
  return friend.blacklisted || friendMatchesRuleList(rules.blacklist, friend);
}

function isFriendWhitelistedByRules(friend: FriendRow, rules: FriendRules): boolean {
  return friend.whitelisted || friendMatchesRuleList(rules.whitelist, friend);
}

type FriendCommandGroup = 'actions' | 'config';

type FriendCommandMenuState = {
  friend: FriendRow;
  group: FriendCommandGroup;
  top: number;
  right: number;
};

type SystemBlockTarget = {
  gid: string;
  displayName: string;
};

type FriendCommandItem = {
  label: string;
  icon: ReactNode;
  disabled: boolean;
  reason: string | null;
  danger?: boolean;
  onSelect: () => void | Promise<void>;
};

function localRuleBlockReason(friend: FriendRow, rules: FriendRules, scope: 'steal' | 'help' | 'mischief'): string | null {
  const blacklisted = isFriendBlacklistedByRules(friend, rules);
  const whitelisted = isFriendWhitelistedByRules(friend, rules);
  if (rules.blacklistEnabled && rules.blacklistScopes.includes(scope) && (blacklisted || friend.maskedBlocked)) {
    return friend.maskedBlocked ? '低等级屏蔽已跳过' : '本地黑名单已跳过';
  }
  if (rules.whitelistEnabled && rules.whitelistScopes.includes(scope) && !whitelisted) {
    return '本地白名单未包含该好友';
  }
  if (friend.protected) return '保护好友不可执行此操作';
  return null;
}

export function SocialView({
  state,
  isQQRuntime = false,
  rankingPreferences = defaultRankingPreferences,
  rankingDataVersions = defaultRankingDataVersions,
  dogGuardState,
  protocolBlockList = [],
  lastActionResult,
  onActionResultConsumed,
  onRefresh,
  onAction,
  onRankings = emptyRankingPage,
  onRefreshVisitors = unavailableVisitorRefresh,
  onSaveRankingPreferences,
  onDogGuardAction,
  onProtocolBlockList,
  onExport,
  onImport,
  onSaveTextFile,
  initialRankingOpen = false,
  initialFeatureOpen = false,
  initialRulesOpen = false,
  initialDogOpen = false,
  initialImportExportOpen = false,
  initialProtocolOpen = false,
}: SocialViewProps) {
  const current = state || emptyState;
  const dogState = dogGuardState || emptyDogGuard;
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<'all' | 'stealable' | 'helpable' | 'blacklist' | 'whitelist' | 'dog'>('all');
  const [friendSortMode, setFriendSortMode] = useState<FriendSortMode>('default');
  const [featureOpen, setFeatureOpen] = useState(initialFeatureOpen);
  const [rulesOpen, setRulesOpen] = useState(initialRulesOpen);
  const [batchRuleType, setBatchRuleType] = useState<'blacklist' | 'whitelist' | null>(null);
  const [selectedRuleItems, setSelectedRuleItems] = useState<string[]>([]);
  const [ruleDraft, setRuleDraft] = useState<FriendRules>(current.rules);
  const [rankingOpen, setRankingOpen] = useState(initialRankingOpen);
  const [dogOpen, setDogOpen] = useState(initialDogOpen);
  const [dogScanIntervalMs, setDogScanIntervalMs] = useState(300);
  const [dogSkipScanned, setDogSkipScanned] = useState(true);
  const [dogExcludeGuardDog, setDogExcludeGuardDog] = useState(false);
  const [importExportOpen, setImportExportOpen] = useState(initialImportExportOpen);
  const [protocolOpen, setProtocolOpen] = useState(initialProtocolOpen);
  const [selectedProtocolGIDs, setSelectedProtocolGIDs] = useState<string[]>([]);
  const [protocolUnblockPending, setProtocolUnblockPending] = useState<string | null>(null);
  const [friendCommandMenu, setFriendCommandMenu] = useState<FriendCommandMenuState | null>(null);
  const [qqProfilePendingGID, setQQProfilePendingGID] = useState<number | null>(null);
  const [systemBlockTarget, setSystemBlockTarget] = useState<SystemBlockTarget | null>(null);
  const [systemBlockPending, setSystemBlockPending] = useState(false);
  const [godRankOpen, setGodRankOpen] = useState(() => Boolean(godRankPayloadFromAction(lastActionResult)));
  const [godRankPayload, setGodRankPayload] = useState<GodRankPayload | null>(() => godRankPayloadFromAction(lastActionResult));
  const [exportText, setExportText] = useState('');
  const [importText, setImportText] = useState('');
  const [visibleActionResult, setVisibleActionResult] = useState<SocialActionResult | null>(lastActionResult || null);
  const previousDogState = useRef({ running: dogState.running, finishedAt: dogState.finishedAt });
  const notifiedDogFinish = useRef<string | undefined>(dogState.finishedAt);
  const onActionResultConsumedRef = useRef(onActionResultConsumed);
  const friendCommandMenuRef = useRef<HTMLDivElement>(null);
  const protocolListKey = protocolBlockList.map((friend) => String(friend.gid)).join('|');

  useEffect(() => {
    if (!lastActionResult) return;
    const nextGodRank = godRankPayloadFromAction(lastActionResult);
    if (nextGodRank) {
      setGodRankPayload(nextGodRank);
      setGodRankOpen(true);
    }
    setVisibleActionResult(lastActionResult);
  }, [lastActionResult]);

  useEffect(() => {
    onActionResultConsumedRef.current = onActionResultConsumed;
  }, [onActionResultConsumed]);

  useEffect(() => () => onActionResultConsumedRef.current?.(), []);

  useEffect(() => {
    if (!visibleActionResult) return;
    const timer = setTimeout(() => setVisibleActionResult(null), SOCIAL_TOAST_AUTO_DISMISS_MS);
    return () => clearTimeout(timer);
  }, [visibleActionResult]);

  useEffect(() => {
    if (rulesOpen) setRuleDraft(current.rules);
  }, [current.rules, rulesOpen]);

  useEffect(() => {
    const currentGIDs = new Set(protocolListKey.split('|').filter(Boolean));
    setSelectedProtocolGIDs((selected) => selected.filter((gid) => currentGIDs.has(gid)));
  }, [protocolListKey]);

  useEffect(() => {
    if (!friendCommandMenu && !systemBlockTarget) return undefined;
    if (typeof document === 'undefined') return undefined;
    function onKeyDown(event: KeyboardEvent) {
      if (event.key !== 'Escape') return;
      if (systemBlockTarget) {
        setSystemBlockTarget(null);
        return;
      }
      setFriendCommandMenu(null);
    }
    function onPointerDown(event: globalThis.MouseEvent) {
      if (friendCommandMenuRef.current?.contains(event.target as Node)) return;
      setFriendCommandMenu(null);
    }
    document.addEventListener('keydown', onKeyDown);
    document.addEventListener('mousedown', onPointerDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      document.removeEventListener('mousedown', onPointerDown);
    };
  }, [friendCommandMenu, systemBlockTarget]);

  useEffect(() => {
    const previous = previousDogState.current;
    previousDogState.current = { running: dogState.running, finishedAt: dogState.finishedAt };
    if (!previous.running || dogState.running) return;
    void onRefresh(false);
    if (!dogState.finishedAt || dogState.finishedAt === notifiedDogFinish.current) return;
    notifiedDogFinish.current = dogState.finishedAt;
    if (dogState.error) {
      setVisibleActionResult({ ok: false, status: 'failed', message: dogState.error });
      return;
    }
    if (dogState.stopRequested) return;
    const currentFriendGIDs = new Set(current.friends.map((friend) => friend.gid).filter((gid) => gid > 0));
    const guardedFriendGIDs = new Set(
      (dogState.results || [])
        .filter((row) => row.hasGuardDog && row.gid > 0 && currentFriendGIDs.has(row.gid))
        .map((row) => row.gid),
    );
    setVisibleActionResult({
      ok: true,
      status: 'ok',
      message: `已扫描到 ${guardedFriendGIDs.size} 名护主犬好友，已自动标记`,
    });
  }, [current.friends, dogState.error, dogState.finishedAt, dogState.results, dogState.running, dogState.stopRequested, onRefresh]);

  const friends = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    return sortFriendsByLevel(current.friends.filter((friend) => {
      const blacklisted = isFriendBlacklistedByRules(friend, current.rules);
      const whitelisted = isFriendWhitelistedByRules(friend, current.rules);
      if (filter === 'stealable' && !friend.stealable) return false;
      if (filter === 'helpable' && !friend.helpable) return false;
      if (filter === 'blacklist' && !blacklisted && !friend.maskedBlocked) return false;
      if (filter === 'whitelist' && !whitelisted) return false;
      if (filter === 'dog' && !friend.hasGuardDog) return false;
      if (!keyword) return true;
      return [friend.displayName, friend.name, friend.remark, String(friend.gid)].some((value) =>
        (value || '').toLowerCase().includes(keyword),
      );
    }), friendSortMode);
  }, [current.friends, current.rules, filter, friendSortMode, query]);

  async function saveRuleExport(listName: RuleExportName, items: string[]) {
    const request = buildRuleExportFileRequest(listName, items);
    setExportText(request.content);
    if (!onSaveTextFile) return;
    try {
      const result = await onSaveTextFile(request);
      setVisibleActionResult({
        ok: true,
        status: 'ok',
        message: result?.path ? `已导出到：${result.path}` : '名单已导出。',
      });
    } catch (error) {
      setVisibleActionResult({
        ok: false,
        status: 'failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  async function exportRuleList(listName: RuleListName) {
    await saveRuleExport(listName, current.rules[listName]);
  }

  async function exportAllLoadedFriends() {
    await saveRuleExport('all', current.friends.map((friend) => String(friend.gid)));
  }

  async function importRuleList(listName: RuleListName) {
    if (!importText.trim()) return;
    await onAction({ action: 'rules_save', rules: buildRulesWithImportedList(current.rules, listName, importText) });
  }

  function openBatchRuleDialog(type: 'blacklist' | 'whitelist') {
    setBatchRuleType(type);
    setSelectedRuleItems([]);
    setFeatureOpen(false);
  }

  function toggleRuleSelection(item: string) {
    setSelectedRuleItems((items) => (items.includes(item) ? items.filter((value) => value !== item) : [...items, item]));
  }

  async function submitBatchRuleRemove() {
    if (!batchRuleType || selectedRuleItems.length === 0) return;
    await onAction({
      action: batchRuleType === 'blacklist' ? 'blacklist_remove_batch' : 'whitelist_remove_batch',
      targets: selectedRuleItems,
    });
    setSelectedRuleItems([]);
  }

  function toggleProtocolSelection(gid: string) {
    setSelectedProtocolGIDs((selected) =>
      selected.includes(gid) ? selected.filter((item) => item !== gid) : [...selected, gid],
    );
  }

  async function submitProtocolUnblock(gid: string) {
    setProtocolUnblockPending(gid);
    try {
      await onAction({ action: 'unblock_friend', target: gid });
    } finally {
      try {
        await onProtocolBlockList?.();
      } finally {
        setProtocolUnblockPending(null);
      }
    }
  }

  async function submitProtocolUnblockBatch() {
    if (selectedProtocolGIDs.length === 0) return;
    const targets = [...selectedProtocolGIDs];
    setProtocolUnblockPending('__batch__');
    try {
      await onAction({ action: 'unblock_friend_batch', targets });
    } finally {
      try {
        setSelectedProtocolGIDs([]);
        await onProtocolBlockList?.();
      } finally {
        setProtocolUnblockPending(null);
      }
    }
  }

  async function saveRuleSettings() {
    await onAction({ action: 'rules_save', rules: ruleDraft });
  }

  function updateRuleDraft(patch: Partial<FriendRules>) {
    setRuleDraft((rules) => ({ ...rules, ...patch }));
  }

  function toggleScope(key: 'blacklistScopes' | 'whitelistScopes', scope: string) {
    setRuleDraft((rules) => {
      if (key === 'whitelistScopes' && rules.blacklistEnabled && rules.blacklistScopes.includes(scope) && !rules.whitelistScopes.includes(scope)) {
        return rules;
      }
      if (key === 'blacklistScopes' && rules.whitelistEnabled && rules.whitelistScopes.includes(scope)) {
        return rules;
      }
      const currentScopes = rules[key] || [];
      const next = currentScopes.includes(scope)
        ? currentScopes.filter((item) => item !== scope)
        : [...currentScopes, scope];
      return { ...rules, [key]: next };
    });
  }

  async function startDogGuardScan() {
    await onDogGuardAction?.({
      action: 'start',
      refresh: true,
      scanIntervalMs: dogScanIntervalMs,
      skipScanned: dogSkipScanned,
      excludeGuardDog: dogExcludeGuardDog,
    });
  }

  function openFriendCommand(event: MouseEvent<HTMLButtonElement>, friend: FriendRow, group: FriendCommandGroup) {
    const rect = event.currentTarget.getBoundingClientRect();
    const next = {
      friend,
      group,
      top: rect.bottom + 6,
      right: Math.max(12, (typeof window === 'undefined' ? rect.right : window.innerWidth - rect.right)),
    };
    setFriendCommandMenu((currentMenu) => (
      currentMenu?.friend.gid === friend.gid && currentMenu.group === group ? null : next
    ));
  }

  async function submitFriendCommand(action: string, friend: FriendRow) {
    if (action === 'view_qq') {
      if (qqProfilePendingGID === friend.gid) return;
      setQQProfilePendingGID(friend.gid);
      try {
        await onAction({ action, target: String(friend.gid) });
      } finally {
        setQQProfilePendingGID(null);
        setFriendCommandMenu(null);
      }
      return;
    }
    setFriendCommandMenu(null);
    await onAction({ action, target: String(friend.gid) });
  }

  function requestSystemBlock(target: SystemBlockTarget) {
    setSystemBlockTarget(target);
  }

  async function submitSystemBlock() {
    if (!systemBlockTarget || systemBlockPending) return;
    setSystemBlockPending(true);
    try {
      await onAction({ action: 'block_friend', target: systemBlockTarget.gid });
      await onRefresh(false);
      await onProtocolBlockList?.();
    } catch (error) {
      setVisibleActionResult({
        ok: false,
        status: 'failed',
        message: error instanceof Error ? error.message : String(error),
      });
    } finally {
      setSystemBlockPending(false);
      setSystemBlockTarget(null);
      setFriendCommandMenu(null);
    }
  }

  function closeGodRank() {
    setGodRankOpen(false);
    onActionResultConsumed?.();
  }

  const commandItems: FriendCommandItem[] = (() => {
    if (!friendCommandMenu) return [];
    const friend = friendCommandMenu.friend;
    if (friendCommandMenu.group === 'actions') {
      const stealReason = localRuleBlockReason(friend, current.rules, 'steal') || (!friend.stealable ? '当前无可偷作物' : null);
      const mischiefReason = localRuleBlockReason(friend, current.rules, 'mischief') || (!friend.mischiefable ? '当前无可捣乱目标' : null);
      const helpReason = localRuleBlockReason(friend, current.rules, 'help') || (!friend.helpable ? '当前无需帮助' : null);
      return [
        ...(isQQRuntime ? [{
          label: qqProfilePendingGID === friend.gid ? '打开QQ中' : '查看好友QQ',
          icon: <Eye size={16} />,
          disabled: qqProfilePendingGID === friend.gid,
          reason: null,
          onSelect: () => submitFriendCommand('view_qq', friend),
        }] : []),
        { label: '查看', icon: <Eye size={16} />, disabled: false, reason: null, onSelect: () => submitFriendCommand('enter', friend) },
        { label: '偷菜', icon: <Swords size={16} />, disabled: Boolean(stealReason), reason: stealReason, onSelect: () => submitFriendCommand('steal', friend) },
        { label: '捣乱', icon: <Bug size={16} />, disabled: Boolean(mischiefReason), reason: mischiefReason, onSelect: () => submitFriendCommand('mischief', friend) },
        { label: '帮助', icon: <HeartHandshake size={16} />, disabled: Boolean(helpReason), reason: helpReason, onSelect: () => submitFriendCommand('help', friend) },
      ];
    }
    const blacklisted = isFriendBlacklistedByRules(friend, current.rules);
    const whitelisted = isFriendWhitelistedByRules(friend, current.rules);
    return [
      {
        label: blacklisted ? '移出本地黑名单' : '加入本地黑名单',
        icon: <Ban size={16} />,
        disabled: false,
        reason: null,
        onSelect: () => submitFriendCommand('blacklist_toggle', friend),
      },
      {
        label: whitelisted ? '移出本地白名单' : '加入本地白名单',
        icon: <Check size={16} />,
        disabled: false,
        reason: null,
        onSelect: () => submitFriendCommand('whitelist_toggle', friend),
      },
      {
        label: friend.protocolBlocked ? '已加入系统黑名单' : '加入系统黑名单',
        icon: <Ban size={16} />,
        disabled: friend.protocolBlocked || systemBlockPending,
        reason: friend.protocolBlocked ? '该好友已在系统黑名单中' : null,
        danger: true,
        onSelect: () => requestSystemBlock({
          gid: String(friend.gid),
          displayName: friend.displayName || friend.name || String(friend.gid),
        }),
      },
    ];
  })();

  return (
    <section className="view-stack fill social-view" data-qq-runtime={isQQRuntime ? 'true' : 'false'}>
      <header className="page-header social-header">
        <div>
          <h1>好友社交</h1>
          <p>{current.message || '好友列表、名单规则、排行榜和访客记录。'}</p>
        </div>
        <div className="header-actions">
          <button className="secondary-button social-action-button" type="button" onClick={() => onRefresh(true)}>
            <RefreshCcw size={16} />
            刷新好友
          </button>
          <button className="secondary-button social-action-button" type="button" onClick={() => setFeatureOpen(true)}>
            <Users size={16} />
            好友功能
          </button>
          <button
            className="secondary-button social-action-button dark"
            type="button"
            onClick={() => setRankingOpen(true)}
          >
            <Trophy size={16} />
            排行榜
          </button>
          <button className="primary-button social-action-button" type="button" onClick={() => setImportExportOpen(true)}>
            <Upload size={16} />
            导入导出
          </button>
        </div>
      </header>

      <div className="social-summary-strip">
        <Metric label="好友" value={current.summary.totalFriends} />
        <Metric label="可偷" value={current.summary.stealableFriends} tone="blue" />
        <Metric label="可帮忙" value={current.summary.helpableFriends} tone="green" />
        <Metric label="本地黑名单" value={current.summary.blacklisted + current.summary.maskedBlocked} tone="amber" />
        <Metric label="本地白名单" value={current.summary.whitelisted} />
        <Metric label="护主犬" value={current.summary.dogGuardCount} tone="red" />
      </div>

      <section className="social-workbench">
        <div className="social-table-toolbar">
          <div>
            <h2>好友列表</h2>
            <span>{friends.length} / {current.friends.length}</span>
          </div>
          <div className="social-filter-row">
            <div className="social-search">
              <ListFilter size={15} />
              <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索好友" />
            </div>
            <select
              aria-label="好友排序"
              className="social-sort-select"
              value={friendSortMode}
              onChange={(event) => setFriendSortMode(event.target.value as FriendSortMode)}
            >
              <option value="default">默认</option>
              <option value="level_asc">等级升序</option>
              <option value="level_desc">等级倒序</option>
            </select>
            <div className="social-filter-chips">
              {[
                ['all', '全部'],
                ['stealable', '可偷'],
                ['helpable', '可帮'],
                ['blacklist', '本地黑名单'],
                ['whitelist', '本地白名单'],
                ['dog', '护主犬'],
              ].map(([key, label]) => (
                <button
                  className={filter === key ? 'social-chip active' : 'social-chip'}
                  key={key}
                  type="button"
                  onClick={() => setFilter(key as typeof filter)}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="social-friend-table">
          <div className="social-friend-row social-friend-head">
            <span>好友</span>
            <span>状态与名单</span>
            <span>可执行</span>
            <span>操作</span>
          </div>
          {friends.map((friend) => {
            const blacklisted = isFriendBlacklistedByRules(friend, current.rules);
            const whitelisted = isFriendWhitelistedByRules(friend, current.rules);
            return (
              <div className="social-friend-row" key={friend.gid}>
                <div className="social-friend-name">
                  <strong>{friend.displayName}</strong>
                  <small>{friend.gid}{friend.level ? ` · Lv.${friend.level}` : ''}</small>
                </div>
                <div className="social-pill-line">
                  {blacklisted && <Pill label="本地黑名单" tone="dark" />}
                  {whitelisted && <Pill label="本地白名单" tone="green" />}
                  {friend.protected && <Pill label="保护" tone="dark" />}
                  {friend.hasGuardDog && <Pill label="护主犬" tone="red" />}
                  {friend.maskedBlocked && <Pill label="低等级屏蔽" tone="amber" />}
                  {!blacklisted && !whitelisted && !friend.protected && !friend.hasGuardDog && !friend.maskedBlocked && <span className="social-muted">普通</span>}
                </div>
                <div className="social-pill-line">
                  {friend.stealable && <Pill label={`偷 ${friend.workCounts.collect || 0}`} tone="blue" />}
                  {friend.helpable && <Pill label="帮忙" tone="green" />}
                  {friend.mischiefable && <Pill label="捣乱" tone="amber" />}
                  {!friend.stealable && !friend.helpable && !friend.mischiefable && <span className="social-muted">暂无可执行</span>}
                </div>
                <div className="social-row-actions">
                  <button
                    className="secondary-button social-command-trigger"
                    type="button"
                    aria-haspopup="menu"
                    aria-expanded={friendCommandMenu?.friend.gid === friend.gid && friendCommandMenu.group === 'actions'}
                    aria-controls={`friend-command-${friend.gid}-actions`}
                    onClick={(event) => openFriendCommand(event, friend, 'actions')}
                  >
                    <span className="social-command-label">操作</span>
                    <ChevronDown size={14} />
                  </button>
                  <button
                    className="secondary-button social-command-trigger config"
                    type="button"
                    aria-haspopup="menu"
                    aria-expanded={friendCommandMenu?.friend.gid === friend.gid && friendCommandMenu.group === 'config'}
                    aria-controls={`friend-command-${friend.gid}-config`}
                    onClick={(event) => openFriendCommand(event, friend, 'config')}
                  >
                    <span className="social-command-label">配置</span>
                    <ChevronDown size={14} />
                  </button>
                </div>
              </div>
            );
          })}
          {friends.length === 0 && <div className="social-empty">暂无好友数据</div>}
        </div>
      </section>

      {friendCommandMenu && (
        <>
          <div className="social-command-menu-backdrop" onClick={() => setFriendCommandMenu(null)} />
          <div
            className="social-command-menu"
            id={`friend-command-${friendCommandMenu.friend.gid}-${friendCommandMenu.group}`}
            ref={friendCommandMenuRef}
            role="menu"
            aria-label={`${friendCommandMenu.friend.displayName}的${friendCommandMenu.group === 'actions' ? '操作' : '配置'}菜单`}
            style={{ top: friendCommandMenu.top, right: friendCommandMenu.right }}
          >
            <div className="social-command-menu-heading">
              <strong>{friendCommandMenu.friend.displayName}</strong>
              <span>GID {friendCommandMenu.friend.gid}</span>
              <button
                className="social-command-menu-close"
                type="button"
                aria-label={`关闭 ${friendCommandMenu.friend.displayName} 的${friendCommandMenu.group === 'actions' ? '操作' : '配置'}菜单`}
                title="关闭"
                onClick={() => setFriendCommandMenu(null)}
              >
                <X size={16} />
              </button>
            </div>
            {commandItems.map((item) => (
              <button
                className={item.danger ? 'social-command-item danger' : 'social-command-item'}
                disabled={item.disabled}
                key={item.label}
                role="menuitem"
                type="button"
                onClick={() => { void item.onSelect(); }}
              >
                {item.icon}
                <span>{item.label}</span>
                {item.reason && <small>{item.reason}</small>}
              </button>
            ))}
          </div>
        </>
      )}

      {visibleActionResult && (
        <div className={visibleActionResult.ok ? 'social-toast ok' : 'social-toast'} role="status" aria-live="polite">
          {visibleActionResult.ok ? <Check size={16} /> : <X size={16} />}
          <span>{visibleActionResult.message}</span>
        </div>
      )}

      {featureOpen && (
        <Dialog title="好友功能" onClose={() => setFeatureOpen(false)}>
          <div className="social-feature-grid">
            <FeatureButton
              icon={<Trophy size={18} />}
              label="查看封神榜"
              actionId="god_rank"
              onClick={() => {
                setFeatureOpen(false);
                onAction({ action: 'god_rank' });
              }}
            />
            <FeatureButton
              icon={<Ban size={18} />}
              label="查看系统拉黑"
              actionId="system_block_list"
              onClick={() => {
                setProtocolOpen(true);
                onProtocolBlockList?.();
                setFeatureOpen(false);
              }}
            />
            <FeatureButton icon={<Sparkles size={18} />} label="清理无效本地黑名单" actionId="cleanup_invalid_rules" onClick={() => onAction({ action: 'cleanup_invalid_rules' })} />
            <FeatureButton icon={<X size={18} />} label="批量移除本地黑名单" actionId="blacklist_remove_batch" onClick={() => openBatchRuleDialog('blacklist')} />
            <FeatureButton icon={<Check size={18} />} label="批量移除本地白名单" actionId="whitelist_remove_batch" onClick={() => openBatchRuleDialog('whitelist')} />
            <FeatureButton
              icon={<ShieldCheck size={18} />}
              label="读取护主犬"
              actionId="dog_guard_scan"
              onClick={() => {
                setDogOpen(true);
                setFeatureOpen(false);
              }}
            />
            <FeatureButton icon={<Shield size={18} />} label="本地名单设置" actionId="rules_save" onClick={() => {
              setRulesOpen(true);
              setFeatureOpen(false);
            }} />
          </div>
        </Dialog>
      )}

      {rulesOpen && (
        <Dialog
          title="本地名单设置"
          wide
          onClose={() => setRulesOpen(false)}
          footer={(
            <>
              <button className="secondary-button social-action-button" type="button" onClick={() => setRulesOpen(false)}>取消</button>
              <button className="primary-button social-action-button" type="button" onClick={saveRuleSettings}>保存设置</button>
            </>
          )}
        >
          <RuleSettings rules={ruleDraft} onChange={updateRuleDraft} onToggleScope={toggleScope} />
        </Dialog>
      )}

      {systemBlockTarget && (
        <Dialog
          title="确认加入系统黑名单"
          onClose={() => !systemBlockPending && setSystemBlockTarget(null)}
          footer={(
            <>
              <button className="secondary-button social-action-button" type="button" disabled={systemBlockPending} onClick={() => setSystemBlockTarget(null)}>取消</button>
              <button className="primary-button social-action-button" type="button" disabled={systemBlockPending} onClick={submitSystemBlock}>
                <Ban size={14} />
                {systemBlockPending ? '加入中' : '确认加入'}
              </button>
            </>
          )}
        >
          <div className="social-confirm-copy">
            <strong>{systemBlockTarget.displayName}</strong>
            <span>GID {systemBlockTarget.gid}</span>
            <p>加入后可在“查看系统拉黑”中解除该好友。</p>
          </div>
        </Dialog>
      )}

      {batchRuleType && (
        <Dialog
          title={batchRuleType === 'blacklist' ? '批量移除本地黑名单' : '批量移除本地白名单'}
          onClose={() => setBatchRuleType(null)}
          footer={(
            <>
              <button className="secondary-button social-action-button" type="button" onClick={() => setBatchRuleType(null)}>取消</button>
              <button className="primary-button social-action-button" type="button" disabled={selectedRuleItems.length === 0} onClick={submitBatchRuleRemove}>移除选中</button>
            </>
          )}
        >
          <BatchRuleRemoveDialog
            actionId={batchRuleType === 'blacklist' ? 'blacklist_remove_batch' : 'whitelist_remove_batch'}
            items={batchRuleType === 'blacklist' ? current.rules.blacklist : current.rules.whitelist}
            selected={selectedRuleItems}
            onToggle={toggleRuleSelection}
            onSelectAll={(items) => setSelectedRuleItems(items)}
            onClear={() => setSelectedRuleItems([])}
          />
        </Dialog>
      )}

      <SocialRankingDialog
        open={rankingOpen}
        dataVersions={rankingDataVersions}
        rankingPreferences={rankingPreferences}
        onRankings={onRankings}
        onRefreshVisitors={onRefreshVisitors}
        onSaveRankingPreferences={onSaveRankingPreferences || keepRankingPreferences}
        onClose={() => setRankingOpen(false)}
        onProtocolBlock={requestSystemBlock}
      />

      {dogOpen && (
        <Dialog title="读取护主犬" wide onClose={() => setDogOpen(false)}>
          <div className="social-dialog-toolbar">
            <button className="primary-button social-action-button" type="button" disabled={dogState.running} onClick={startDogGuardScan}>
              <RefreshCcw size={16} />
              {dogState.running ? '扫描中' : '开始扫描'}
            </button>
            <button className="secondary-button social-action-button" type="button" onClick={() => onDogGuardAction?.({ action: 'stop' })}>停止</button>
            <button className="secondary-button social-action-button" type="button" onClick={() => onDogGuardAction?.({ action: 'clear' })}>清空</button>
            <span className="social-muted">{dogState.scanned}/{dogState.total} · {dogState.hasGuardDogCount}</span>
          </div>
          <DogGuardOptionsPanel
            scanIntervalMs={dogScanIntervalMs}
            skipScanned={dogSkipScanned}
            excludeGuardDog={dogExcludeGuardDog}
            onScanIntervalMs={setDogScanIntervalMs}
            onSkipScanned={setDogSkipScanned}
            onExcludeGuardDog={setDogExcludeGuardDog}
          />
          {dogState.current && (
            <div className="social-inline-status">当前读取：{dogState.current.displayName || dogState.current.name || dogState.current.gid}</div>
          )}
          <div className="social-mini-table">
            {(dogState.results || []).map((row) => (
              <div className="social-mini-row" key={row.gid}>
                <strong>{row.displayName || row.name || row.gid}</strong>
                <span>{row.hasGuardDog ? '有护主犬' : row.error || '未发现'}</span>
              </div>
            ))}
            {(dogState.results || []).length === 0 && <div className="social-empty">暂无护主犬扫描记录</div>}
          </div>
        </Dialog>
      )}

      {protocolOpen && (
        <Dialog
          title="系统黑名单"
          onClose={() => setProtocolOpen(false)}
          footer={(
            <>
              <button className="secondary-button social-action-button" type="button" onClick={() => setProtocolOpen(false)}>取消</button>
              <button
                className="primary-button social-action-button"
                type="button"
                disabled={protocolUnblockPending !== null || selectedProtocolGIDs.length === 0}
                onClick={submitProtocolUnblockBatch}
              >
                <Unlock size={14} />
                {protocolUnblockPending === '__batch__' ? '解除中' : '解除选中'}
              </button>
            </>
          )}
        >
          <div className="social-dialog-toolbar social-protocol-toolbar">
            <span className="social-muted">已选择 {selectedProtocolGIDs.length} / {protocolBlockList.length}</span>
            <div className="social-protocol-toolbar-actions">
              <button
                className="secondary-button social-action-button compact"
                type="button"
                disabled={protocolUnblockPending !== null || protocolBlockList.length === 0}
                onClick={() => setSelectedProtocolGIDs(protocolBlockList.map((friend) => String(friend.gid)))}
              >
                全选当前列表
              </button>
              <button
                className="secondary-button social-action-button compact"
                type="button"
                disabled={protocolUnblockPending !== null || selectedProtocolGIDs.length === 0}
                onClick={() => setSelectedProtocolGIDs([])}
              >
                清空选择
              </button>
            </div>
          </div>
          <div className="social-mini-table">
            {protocolBlockList.map((friend) => {
              const gid = String(friend.gid);
              const rowPending = protocolUnblockPending === gid;
              return (
                <div className="social-mini-row social-protocol-row" key={friend.gid}>
                  <label className="social-protocol-select">
                    <input
                      aria-label={`选择 ${friend.displayName || friend.name || gid}`}
                      type="checkbox"
                      checked={selectedProtocolGIDs.includes(gid)}
                      disabled={protocolUnblockPending !== null}
                      onChange={() => toggleProtocolSelection(gid)}
                    />
                  </label>
                  <strong>{friend.displayName || friend.name || friend.gid}</strong>
                  <span>{friend.gid}</span>
                  <em>系统拉黑</em>
                  <button
                    className="secondary-button social-action-button compact"
                    type="button"
                    disabled={protocolUnblockPending !== null}
                    onClick={() => submitProtocolUnblock(gid)}
                  >
                    <Unlock size={14} />
                    {rowPending ? '解除中' : '解除系统拉黑'}
                  </button>
                </div>
              );
            })}
            {protocolBlockList.length === 0 && <div className="social-empty">暂无系统拉黑好友</div>}
          </div>
        </Dialog>
      )}

      {godRankOpen && godRankPayload && (
        <Dialog title="封神榜" wide onClose={closeGodRank}>
          <div className="social-dialog-toolbar">
            <strong>查询到的封禁好友</strong>
            <span className="social-muted">{godRankPayload.count ?? godRankPayload.list.length} 人</span>
          </div>
          <div className="social-mini-table">
            {godRankPayload.list.map((row, index) => {
              const name = row.displayName || row.remark || row.name || row.gid || '未知好友';
              const rank = row.displayRank || row.rank || index + 1;
              return (
                <div className="social-mini-row" key={`${row.gid || name}-${index}`}>
                  <strong>{name}</strong>
                  <span>{row.gid || '-'}</span>
                  <em>第 {rank}{row.level ? ` · Lv.${row.level}` : ''} · 可偷 {row.stealableCount || 0} · 可帮 {row.helpableCount || 0}</em>
                </div>
              );
            })}
            {godRankPayload.list.length === 0 && <div className="social-empty">暂无封禁好友</div>}
          </div>
        </Dialog>
      )}

      {importExportOpen && (
        <Dialog title="名单导入导出" wide onClose={() => setImportExportOpen(false)}>
          <div className="social-import-export">
            <div className="social-import-export-copy">
              导入本地黑名单或本地白名单 gid，或导出名单和全部已加载好友 gid。
            </div>
            <section>
              <div className="social-import-export-actions">
                <button className="secondary-button social-action-button social-import-export-button blue" type="button" onClick={() => importRuleList('blacklist')}>
                  <Upload size={16} />
                  导入本地黑名单
                </button>
                <button className="secondary-button social-action-button social-import-export-button blue" type="button" onClick={() => importRuleList('whitelist')}>
                  <Upload size={16} />
                  导入本地白名单
                </button>
                <button className="secondary-button social-action-button social-import-export-button amber" type="button" onClick={() => exportRuleList('blacklist')}>
                  <Download size={16} />
                  导出本地黑名单
                </button>
                <button className="secondary-button social-action-button social-import-export-button blue" type="button" onClick={() => exportRuleList('whitelist')}>
                  <Download size={16} />
                  导出本地白名单
                </button>
                <button className="secondary-button social-action-button social-import-export-button dark" type="button" onClick={exportAllLoadedFriends}>
                  <Download size={16} />
                  导出全部
                </button>
              </div>
              <textarea value={importText} onChange={(event) => setImportText(event.target.value)} placeholder="在这里粘贴要导入的 gid 列表，或粘贴旧版 JSON 导出内容。" />
            </section>
            <section>
              <textarea readOnly value={exportText} placeholder="点击导出按钮后，这里会生成可复制的 gid 列表。" />
            </section>
          </div>
        </Dialog>
      )}
    </section>
  );
}

function Metric({ label, value, tone = 'plain' }: { label: string; value: number; tone?: string }) {
  return (
    <div className={`social-metric ${tone}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function Pill({ label, tone }: { label: string; tone: string }) {
  return <span className={`social-pill ${tone}`}>{label}</span>;
}

function Dialog({ title, children, footer, onClose, wide = false }: { title: string; children: ReactNode; footer?: ReactNode; onClose: () => void; wide?: boolean }) {
  const titleID = `social-dialog-${title}`;
  return (
    <div className="dialog-backdrop social-dialog-backdrop">
      <section className={wide ? 'social-dialog wide' : 'social-dialog'} role="dialog" aria-modal="true" aria-labelledby={titleID}>
        <header>
          <h2 id={titleID}>{title}</h2>
          <button className="icon-button light" type="button" onClick={onClose} aria-label={`关闭${title}`} title="关闭">
            <X size={16} />
          </button>
        </header>
        <div className="social-dialog-body">{children}</div>
        {footer && <footer className="social-dialog-footer">{footer}</footer>}
      </section>
    </div>
  );
}

function FeatureButton({ icon, label, actionId, onClick }: { icon: ReactNode; label: string; actionId?: string; onClick: () => void }) {
  return (
    <button className="social-feature-button" type="button" data-action-id={actionId} onClick={onClick}>
      {icon}
      <span>{label}</span>
    </button>
  );
}

function BatchRuleRemoveDialog({
  actionId,
  items,
  selected,
  onToggle,
  onSelectAll,
  onClear,
}: {
  actionId: string;
  items: string[];
  selected: string[];
  onToggle: (item: string) => void;
  onSelectAll: (items: string[]) => void;
  onClear: () => void;
}) {
  return (
    <div className="social-batch-panel" data-action-id={actionId}>
      <div className="social-dialog-toolbar">
        <button className="secondary-button social-action-button" type="button" onClick={() => onSelectAll(items)}>全选当前列表</button>
        <button className="secondary-button social-action-button" type="button" onClick={onClear}>解除选中</button>
        <span className="social-muted">已选 {selected.length} / {items.length}</span>
      </div>
      <div className="social-mini-table">
        {items.map((item) => (
          <label className="social-check-row" key={item}>
            <input type="checkbox" checked={selected.includes(item)} onChange={() => onToggle(item)} />
            <strong>{item}</strong>
            <span>{/^\d+$/.test(item) ? 'gid 规则' : '文本规则'}</span>
          </label>
        ))}
        {items.length === 0 && <div className="social-empty">当前列表为空</div>}
      </div>
    </div>
  );
}

function RuleSettings({
  rules,
  onChange,
  onToggleScope,
}: {
  rules: FriendRules;
  onChange: (patch: Partial<FriendRules>) => void;
  onToggleScope: (key: 'blacklistScopes' | 'whitelistScopes', scope: string) => void;
}) {
  const disabledWhitelistScopes = new Set<string>();
  const disabledBlacklistScopes = new Set<string>();
  for (const [scope] of scopeOptions) {
    const whitelistActive = rules.whitelistEnabled && rules.whitelistScopes.includes(scope);
    const blacklistActive = rules.blacklistEnabled && rules.blacklistScopes.includes(scope);
    const owner = whitelistActive ? 'whitelist' : blacklistActive ? 'blacklist' : null;
    if (owner === 'blacklist') disabledWhitelistScopes.add(scope);
    if (owner === 'whitelist') disabledBlacklistScopes.add(scope);
  }
  return (
    <div className="social-rule-settings">
      <section className="social-rule-editor">
        <label className="social-switch-row">
          <input type="checkbox" checked={rules.whitelistEnabled} onChange={(event) => onChange({ whitelistEnabled: event.target.checked })} />
          启用本地白名单
        </label>
        <div className="social-scope-row">
          {scopeOptions.map(([scope, label]) => {
            const disabled = disabledWhitelistScopes.has(scope);
            return (
              <label className={disabled ? 'disabled' : undefined} data-scope={`whitelist-${scope}`} key={scope} title={disabled ? '本地黑名单已启用该范围，本地白名单不可选' : undefined}>
                <input
                  type="checkbox"
                  checked={rules.whitelistScopes.includes(scope)}
                  disabled={disabled}
                  onChange={() => onToggleScope('whitelistScopes', scope)}
                />
                {label}
              </label>
            );
          })}
        </div>
        {disabledWhitelistScopes.size > 0 && <div className="social-rule-note">本地黑名单已启用该范围，本地白名单不可选</div>}
        <label>
          本地白名单
          <textarea value={listToText(rules.whitelist)} onChange={(event) => onChange({ whitelist: textToList(event.target.value) })} />
        </label>
      </section>
      <section className="social-rule-editor">
        <label className="social-switch-row">
          <input type="checkbox" checked={rules.blacklistEnabled} onChange={(event) => onChange({ blacklistEnabled: event.target.checked })} />
          启用本地黑名单
        </label>
        <div className="social-scope-row">
          {scopeOptions.map(([scope, label]) => {
            const disabled = disabledBlacklistScopes.has(scope);
            return (
              <label className={disabled ? 'disabled' : undefined} data-scope={`blacklist-${scope}`} key={scope} title={disabled ? '本地白名单已启用该范围，本地黑名单不可选' : undefined}>
                <input
                  type="checkbox"
                  checked={rules.blacklistScopes.includes(scope)}
                  disabled={disabled}
                  onChange={() => onToggleScope('blacklistScopes', scope)}
                />
                {label}
              </label>
            );
          })}
        </div>
        {disabledBlacklistScopes.size > 0 && <div className="social-rule-note">本地白名单已启用该范围，本地黑名单不可选</div>}
        <label>
          本地黑名单
          <textarea value={listToText(rules.blacklist)} onChange={(event) => onChange({ blacklist: textToList(event.target.value) })} />
        </label>
      </section>
      <section className="social-rule-editor social-rule-editor-compact">
        <label className="social-switch-row">
          <input type="checkbox" checked={rules.maskedBlacklist} onChange={(event) => onChange({ maskedBlacklist: event.target.checked })} />
          低等级屏蔽
        </label>
        <label>
          最大等级
          <input
            type="number"
            min={1}
            value={rules.maskedMaxLevel}
            onChange={(event) => onChange({ maskedMaxLevel: Math.max(1, Number(event.target.value) || 1) })}
          />
        </label>
      </section>
    </div>
  );
}

function DogGuardOptionsPanel({
  scanIntervalMs,
  skipScanned,
  excludeGuardDog,
  onScanIntervalMs,
  onSkipScanned,
  onExcludeGuardDog,
}: {
  scanIntervalMs: number;
  skipScanned: boolean;
  excludeGuardDog: boolean;
  onScanIntervalMs: (value: number) => void;
  onSkipScanned: (value: boolean) => void;
  onExcludeGuardDog: (value: boolean) => void;
}) {
  return (
    <div className="social-dog-options">
      <label>
        扫描间隔
        <input type="number" min={0} value={scanIntervalMs} onChange={(event) => onScanIntervalMs(Number(event.target.value) || 0)} />
      </label>
      <label className="social-switch-row">
        <input type="checkbox" checked={skipScanned} onChange={(event) => onSkipScanned(event.target.checked)} />
        跳过已扫描
      </label>
      <label className="social-switch-row">
        <input type="checkbox" checked={excludeGuardDog} onChange={(event) => onExcludeGuardDog(event.target.checked)} />
        排除已发现护主犬
      </label>
    </div>
  );
}

function RuleList({ title, items }: { title: string; items: string[] }) {
  return (
    <section className="social-rule-card">
      <strong>{title}</strong>
      <div>
        {items.length > 0 ? items.map((item) => <span key={item}>{item}</span>) : <em>空</em>}
      </div>
    </section>
  );
}
