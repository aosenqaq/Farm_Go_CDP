import { CheckSquare, ChevronDown, Droplet, Filter, Library, Loader2, Map as MapIcon, ReceiptText, Shovel, Sprout, Warehouse, X } from 'lucide-react';
import type { ReactNode } from 'react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

import { FallbackImage } from '../components/FallbackImage';
import { PollApiError, pollLand } from '../lib/pollApi';
import { createPollingController } from '../lib/pollingController';
import type { PollingController } from '../lib/pollingController';
import { FarmAtlasBuyLockedCrops, FarmAtlasBuyLockedPreview, FarmAtlasPreview, FarmCropAnalytics, FarmFertilizeLand, FarmLandRush, FarmShovelLands, FarmWarehouseRefresh, FarmWarehouseSell, FarmWarehouseSellRecords, RunFarmAutomationTask, SaveWarehouseAutoSellSettings, WarehouseAutoSellSettings as LoadWarehouseAutoSellSettings } from '../../wailsjs/go/main/App';
import { mergeLandDetailsDelta } from './lib/landDetailsPolling';
import type { farm, storage } from '../../wailsjs/go/models';

type AssetsTab = 'crops' | 'lands' | 'warehouse' | 'atlas';
type AtlasCategory = 'crop' | 'mutation';

type CropAnalyticsPayloadLike = {
  source?: string;
  items?: farm.CropAnalyticsItem[];
  sort?: string;
  requestedMaxLevel?: number;
  effectiveMaxLevel?: number;
  levelSource?: string;
  strategies?: farm.PlantStrategyMode[];
  recommendations?: PlantRecommendationLike[];
  runtimeError?: string;
  error?: string;
};

type PlantRecommendationLike = {
  value: string;
  label: string;
  recommended?: farm.CropAnalyticsItem;
  currentRecommended?: farm.CropAnalyticsItem;
  currentSource?: string;
  theoreticalRecommended?: farm.CropAnalyticsItem;
};

type CropRankingStrategy = 'highest_level' | 'max_exp' | 'max_fert_exp' | 'max_profit' | 'max_fert_profit';

type AtlasPreviewPayloadLike = {
  source?: string;
  status?: string;
  message?: string;
  refreshEnabled?: boolean;
  buyEnabled?: boolean;
  sections?: AtlasSectionLike[];
  error?: string;
};

type AtlasSectionLike = {
  id: string;
  label: string;
  items?: AtlasItemLike[];
  total: number;
  summary?: {
    total: number;
    unlocked: number;
    locked: number;
  };
};

type AtlasItemLike = {
  id: number;
  name: string;
  seedId: number;
  fruitId: number;
  groupName?: string;
  fruitType?: number;
  fruitLayer?: number;
  fruitRarity?: number;
  progress?: number;
  level: number;
  seasons: number;
  growTime: number;
  locked?: boolean;
  unlocked?: boolean;
  canUpgrade?: boolean;
  isNew?: boolean;
  sort?: number;
  atlasType?: string;
  imageUrl?: string;
};

type AtlasPurchasePreviewPayloadLike = {
  ok?: boolean;
  error?: string;
  level?: {
    effectiveMaxLevel?: number;
    levelSource?: string;
  };
  plan?: {
    purchases?: Array<{
      seedId: number;
      seedName: string;
      goodsId: number;
      price: number;
      count: number;
      requiredLevel: number;
    }>;
    skipped?: Array<{ name?: string; seedId?: number; reason: string }>;
    summary?: {
      lockedCropCount?: number;
      purchasable?: number;
      skipped?: number;
      countPerSeed?: number;
      effectiveLevel?: number;
    };
  };
};

type AtlasPurchaseItemLike = {
  seedId: number;
  seedName: string;
  goodsId: number;
  price: number;
  count: number;
  requiredLevel: number;
};

type RuntimeActionGateLike = {
  id: string;
  label: string;
  enabled: boolean;
  reason: string;
};

type LandDetailsPayloadLike = {
	status?: string;
	message?: string;
	revision?: string;
  farmType?: string;
  totalGrids?: number;
  lands?: LandDetailsItemLike[];
  actions?: RuntimeActionGateLike[];
	runtimeError?: string;
};

type LandDetailsDeltaLike = {
	full?: boolean;
	revision?: string;
	status?: string;
	message?: string;
	farmType?: string;
	totalGrids?: number;
	lands?: LandDetailsItemLike[];
	removedLandIds?: number[];
	actions?: RuntimeActionGateLike[];
	runtimeError?: string;
};

type LandRushPayloadLike = {
  landIds: number[];
  rushThresholdSec: number;
  harvestLinkEnabled: boolean;
  continuousRushEnabled: boolean;
  fertilizerMode: 'normal' | 'organic';
};

type LandCardAction = 'fertilize' | 'shovel';
type LandCardFertilizerMode = 'normal' | 'organic';
type LandQuickAutomationTask = 'own_base' | 'own_collect' | 'own_plant';
type LandQuickActionMenuItem = {
  id: string;
  label: string;
  icon: ReactNode;
  enabled: boolean;
  reason: string;
  danger?: boolean;
  onSelect: () => void;
};

type WarehousePayloadLike = {
  status?: string;
  message?: string;
  items?: WarehouseItemLike[];
  actions?: RuntimeActionGateLike[];
  summary?: WarehouseSummaryLike;
  runtimeError?: string;
};

type WarehouseSellPayloadLike = {
  ok?: boolean;
  error?: string;
  warehouse?: WarehousePayloadLike;
  sell?: Record<string, unknown> & { record?: WarehouseSellRecordLike };
};

type WarehouseSellRecordItemLike = {
  itemId?: number;
  name?: string;
  count?: number;
  unitPrice?: number;
  amount?: number;
};

type WarehouseSellRecordLike = {
  id?: string;
  dateKey?: string;
  occurredAt?: string;
  mode?: string;
  itemKinds?: number;
  totalCount?: number;
  totalAmount?: number;
  items?: WarehouseSellRecordItemLike[];
};

type WarehouseSellToastSummary = {
  totalCount: number;
  times: number;
  totalAmount: number;
};

type WarehouseAutoSellSettings = {
  enabled: boolean;
  intervalMinute: number;
  categories: string[];
};

type LandDetailsItemLike = {
  id: string;
  landId: number;
  landLevel?: number;
  seedId?: number;
  landType?: string;
  landTypeLabel?: string;
  plantName?: string;
  displayPlantName?: string;
  imageUrl?: string;
  hasMutation?: boolean;
  mutationLabel?: string;
  mutationIconUrl?: string;
  mutationImageUrl?: string;
  mutationTypes?: Array<{ typeId?: number; name: string; iconUrl?: string }>;
  status: string;
  statusLabel: string;
  matureInSec?: number;
  matureAtMs?: number;
  matureEtaText?: string;
  currentSeason?: number;
  totalSeason?: number;
  isMultiSeason?: boolean;
  landSize?: number;
  occupancyPlantSize?: number;
  occupancyAnchorLandId?: number;
  occupiedByMultiTilePlant?: boolean;
  needWater?: boolean;
  needWeed?: boolean;
  needBug?: boolean;
  needGoldenBug?: boolean;
  needEraseDead?: boolean;
  canHarvest: boolean;
};

type WarehouseItemLike = {
  id: string;
  itemId: number;
  name: string;
  count: number;
  category: string;
  categoryLabel: string;
  imageUrl?: string;
  canSell: boolean;
  locked: boolean;
  estimatedSellPrice: number;
};

type WarehouseCategorySummaryLike = {
  key: string;
  label: string;
  distinct: number;
  count: number;
};

type WarehouseSummaryLike = {
  totalDistinct: number;
  totalCount: number;
  sellableDistinct: number;
  sellableCount: number;
  estimatedAllSellPrice: number;
  categoryList?: WarehouseCategorySummaryLike[];
};

const warehouseCategoryOptions: WarehouseCategorySummaryLike[] = [
  { key: 'fruit', label: '果实', distinct: 0, count: 0 },
  { key: 'mutation', label: '超变果实', distinct: 0, count: 0 },
  { key: 'seed', label: '种子', distinct: 0, count: 0 },
  { key: 'tool', label: '道具', distinct: 0, count: 0 },
];

const hiddenWarehouseItemNames = new Set([
  '普通化肥容器',
  '有机化肥容器',
  '种植经验',
  '普通收藏点',
  '典藏收藏点',
  '金币',
  '点券',
  '金豆',
  '金豆豆',
]);

const defaultWarehouseAutoSellSettings: WarehouseAutoSellSettings = {
  enabled: false,
  intervalMinute: 60,
  categories: ['fruit'],
};

type AssetsLandViewProps = {
  initialTab?: AssetsTab;
  initialAtlasCategory?: AtlasCategory;
  initialCropAnalytics?: CropAnalyticsPayloadLike;
  initialAtlasPreview?: AtlasPreviewPayloadLike;
  initialLandDetails?: LandDetailsPayloadLike;
  initialWarehouse?: WarehousePayloadLike;
};

const tabs: Array<{ id: AssetsTab; label: string; icon: ReactNode }> = [
  { id: 'crops', label: '作物分析', icon: <Sprout size={16} /> },
  { id: 'lands', label: '土地详情', icon: <MapIcon size={16} /> },
  { id: 'warehouse', label: '仓库', icon: <Warehouse size={16} /> },
  { id: 'atlas', label: '图鉴', icon: <Library size={16} /> },
];

export function AssetsLandView({
  initialTab = 'crops',
  initialAtlasCategory = 'crop',
  initialCropAnalytics,
  initialAtlasPreview,
  initialLandDetails,
  initialWarehouse,
}: AssetsLandViewProps) {
  const [activeTab, setActiveTab] = useState<AssetsTab>(initialTab);
  const [activeAtlasCategory, setActiveAtlasCategory] = useState<AtlasCategory>(initialAtlasCategory);
  const [cropAnalytics, setCropAnalytics] = useState<CropAnalyticsPayloadLike | undefined>(initialCropAnalytics);
  const [atlasPreview, setAtlasPreview] = useState<AtlasPreviewPayloadLike | undefined>(initialAtlasPreview);
  const [atlasPurchasePreview, setAtlasPurchasePreview] = useState<AtlasPurchasePreviewPayloadLike | null>(null);
  const [atlasPurchaseOpen, setAtlasPurchaseOpen] = useState(false);
  const [atlasPurchaseNotice, setAtlasPurchaseNotice] = useState('');
  const [landDetails, setLandDetails] = useState<LandDetailsPayloadLike | undefined>(initialLandDetails);
  const [warehouse, setWarehouse] = useState<WarehousePayloadLike | undefined>(initialWarehouse);
  const [loading, setLoading] = useState(!initialCropAnalytics);
  const [atlasLoading, setAtlasLoading] = useState(!initialAtlasPreview);
  const [atlasPurchaseLoading, setAtlasPurchaseLoading] = useState(false);
  const [landLoading, setLandLoading] = useState(!initialLandDetails);
  const [warehouseLoading, setWarehouseLoading] = useState(false);
  const [warehouseSelling, setWarehouseSelling] = useState(false);
  const [warehouseSellToast, setWarehouseSellToast] = useState<WarehouseSellToastSummary | null>(null);
  const [landRushOpen, setLandRushOpen] = useState(false);
  const [landRushBusy, setLandRushBusy] = useState(false);
  const [landActionBusyKey, setLandActionBusyKey] = useState('');
  const [bulkFertilizeBusyMode, setBulkFertilizeBusyMode] = useState<'normal' | 'organic' | ''>('');
  const [landQuickActionBusy, setLandQuickActionBusy] = useState<LandQuickAutomationTask | 'shovel' | ''>('');
  const [landActionNotice, setLandActionNotice] = useState('');
  const [error, setError] = useState('');
  const [atlasError, setAtlasError] = useState('');
	const [landError, setLandError] = useState('');
  const [warehouseError, setWarehouseError] = useState('');
  const [nowMs, setNowMs] = useState(() => Date.now());
	const landRevision = useRef(initialLandDetails?.revision || '');
	const landPollingRef = useRef<PollingController | null>(null);

  useEffect(() => {
    if (initialCropAnalytics) return;
    let active = true;
    setLoading(true);
    FarmCropAnalytics()
      .then((payload) => {
        if (!active) return;
        setCropAnalytics(payload);
        setError(payload.error || '');
      })
      .catch((err) => {
        if (!active) return;
        setError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [initialCropAnalytics]);

  useEffect(() => {
    if (initialAtlasPreview) return;
    let active = true;
    setAtlasLoading(true);
    FarmAtlasPreview()
      .then((payload) => {
        if (!active) return;
        setAtlasPreview(payload);
        setAtlasError(payload.error || '');
      })
      .catch((err) => {
        if (!active) return;
        setAtlasError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (active) setAtlasLoading(false);
      });
    return () => {
      active = false;
    };
  }, [initialAtlasPreview]);

	const refreshLandDetails = useCallback(() => {
		landRevision.current = '';
    const polling = landPollingRef.current;
    if (!polling) return;
    setLandLoading(true);
    setLandError('');
		polling.stop();
		polling.start();
	}, []);

  useEffect(() => {
    if (activeTab !== 'lands') return;
    const timer = setInterval(() => setNowMs(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [activeTab]);

	useEffect(() => {
		if (activeTab !== 'lands') return;
		landRevision.current = initialLandDetails?.revision || '';
    setLandLoading(true);
    setLandError('');
    const polling = createPollingController<LandDetailsDeltaLike>({
      intervalMs: 5_000,
      read: (signal) => pollLand(landRevision.current, signal) as Promise<LandDetailsDeltaLike>,
      apply: (delta) => {
				landRevision.current = delta.revision || landRevision.current;
				setLandDetails((current) => mergeLandDetailsDelta(
					current ? { ...current, lands: current.lands || [] } : undefined,
					{
						...delta,
						full: Boolean(delta.full),
						revision: delta.revision || landRevision.current,
						lands: delta.lands || [],
						removedLandIds: delta.removedLandIds || [],
					},
				) as LandDetailsPayloadLike);
        setLandLoading(false);
        setLandError('');
      },
      reportError: (error) => {
        setLandLoading(false);
        setLandError(error instanceof Error ? error.message : String(error));
      },
      shouldStop: (error) => error instanceof PollApiError && error.status === 401,
    });
		landPollingRef.current = polling;
		polling.start();

		return () => {
			if (landPollingRef.current === polling) landPollingRef.current = null;
			polling.stop();
		};
	}, [activeTab, initialLandDetails]);

  useEffect(() => {
    if (!warehouseSellToast) return;
    const timer = window.setTimeout(() => setWarehouseSellToast(null), 3600);
    return () => window.clearTimeout(timer);
  }, [warehouseSellToast]);

  function refreshWarehouse() {
    setWarehouseLoading(true);
    setWarehouseError('');
    FarmWarehouseRefresh()
      .then((payload) => setWarehouse(payload))
      .catch((err) => setWarehouseError(err instanceof Error ? err.message : String(err)))
      .finally(() => setWarehouseLoading(false));
  }

  function sellWarehouse(itemKeys: string[]) {
    if (warehouseSelling || itemKeys.length === 0) return;
    setWarehouseSelling(true);
    setWarehouseError('');
    FarmWarehouseSell({ itemKeys, mode: 'manual' } as any)
      .then((payload: WarehouseSellPayloadLike) => {
        if (payload?.ok === false) {
          throw new Error(payload.error || '仓库出售失败');
        }
        if (payload?.warehouse) {
          setWarehouse(payload.warehouse);
        }
        const summary = buildWarehouseSellToastSummary(payload);
        if (summary.totalCount > 0) {
          setWarehouseSellToast(summary);
        }
      })
      .catch((err) => setWarehouseError(err instanceof Error ? err.message : String(err)))
      .finally(() => setWarehouseSelling(false));
  }

  function refreshAtlasPreview() {
    setAtlasLoading(true);
    setAtlasError('');
    setAtlasPurchasePreview(null);
    setAtlasPurchaseOpen(false);
    setAtlasPurchaseNotice('');
    FarmAtlasPreview()
      .then((payload) => {
        setAtlasPreview(payload);
        setAtlasError(payload.error || '');
      })
      .catch((err) => setAtlasError(err instanceof Error ? err.message : String(err)))
      .finally(() => setAtlasLoading(false));
  }

  function scanAtlasLockedCrops() {
    const cropItems = atlasPreview?.sections?.find((section) => section.id === 'crop')?.items || [];
    if (atlasPurchaseLoading || cropItems.length === 0) return;
    setAtlasPurchaseLoading(true);
    setAtlasError('');
    setAtlasPurchaseNotice('');
    FarmAtlasBuyLockedPreview({ items: cropItems, countPerSeed: 1 })
      .then((payload: AtlasPurchasePreviewPayloadLike) => {
        if (payload?.ok === false) {
          throw new Error(payload.error || '图鉴购买扫描失败');
        }
        setAtlasPurchasePreview(payload);
        if ((payload.plan?.purchases || []).length > 0) {
          setAtlasPurchaseOpen(true);
        } else {
          setAtlasPurchaseNotice('没有可购买的未解锁作物种子');
        }
      })
      .catch((err) => setAtlasError(err instanceof Error ? err.message : String(err)))
      .finally(() => setAtlasPurchaseLoading(false));
  }

  function purchaseAtlasLockedCrops() {
    const cropItems = atlasPreview?.sections?.find((section) => section.id === 'crop')?.items || [];
    if (atlasPurchaseLoading || cropItems.length === 0) return;
    setAtlasPurchaseLoading(true);
    setAtlasError('');
    FarmAtlasBuyLockedCrops({ items: cropItems, countPerSeed: 1 })
      .then((payload: AtlasPurchasePreviewPayloadLike) => {
        if (payload?.ok === false) {
          throw new Error(payload.error || '图鉴种子购买失败');
        }
        setAtlasPurchasePreview(payload);
        setAtlasPurchaseOpen(false);
        refreshAtlasPreview();
      })
      .catch((err) => setAtlasError(err instanceof Error ? err.message : String(err)))
      .finally(() => setAtlasPurchaseLoading(false));
  }

  function submitLandRush(input: LandRushPayloadLike) {
    if (landRushBusy || bulkFertilizeBusyMode || landActionBusyKey || landQuickActionBusy) return;
    setLandRushBusy(true);
    setLandError('');
    setLandActionNotice('');
    FarmLandRush(input as any)
      .then((payload: any) => {
        if (payload?.ok === false) {
          throw new Error(payload.error || '一键催熟提交失败');
        }
        setLandRushOpen(false);
        refreshLandDetails();
      })
      .catch((err) => setLandError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLandRushBusy(false));
  }

  function submitBulkFertilizer(mode: LandCardFertilizerMode) {
    if (landRushBusy || bulkFertilizeBusyMode || landActionBusyKey || landQuickActionBusy) return;
    if (landDetails?.farmType === 'friend') {
      setLandError('好友农场不支持一键施肥');
      return;
    }
    const landIds = fertilizerTargetLandIds(landDetails?.lands || []);
    if (landIds.length === 0) {
      setLandError('没有生长中的地块可施肥');
      return;
    }
    setBulkFertilizeBusyMode(mode);
    setLandError('');
    setLandActionNotice('');
    FarmLandRush({
      landIds,
      fertilizerMode: mode,
      fertilizerSubmissionScope: 'manual',
      harvestLinkEnabled: false,
      continuousRushEnabled: false,
      rushThresholdSec: 30 * 24 * 3600,
    } as any)
      .then((payload: any) => {
        if (payload?.ok === false) {
          throw new Error(payload.error || (mode === 'organic' ? '一键有机肥提交失败' : '一键无机肥提交失败'));
        }
        refreshLandDetails();
      })
      .catch((err) => setLandError(err instanceof Error ? err.message : String(err)))
      .finally(() => setBulkFertilizeBusyMode(''));
  }

  function runLandCardAction(action: LandCardAction, land: LandDetailsItemLike, mode?: LandCardFertilizerMode) {
    const landId = landRushLandId(land);
    if (!landId || landRushBusy || bulkFertilizeBusyMode || landActionBusyKey || landQuickActionBusy) return;
    const busyKey = `${action}:${landId}:${mode || ''}`;
    setLandActionBusyKey(busyKey);
    setLandError('');
    setLandActionNotice('');
    const request = action === 'fertilize'
      ? FarmFertilizeLand({ landId, type: mode || 'normal' } as any)
      : FarmShovelLands({ landIds: [landId] } as any);
    request
      .then((payload: any) => {
        if (payload?.ok === false) {
          throw new Error(payload.error || (action === 'fertilize' ? '催熟提交失败' : '铲除提交失败'));
        }
        refreshLandDetails();
      })
      .catch((err) => setLandError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLandActionBusyKey(''));
  }

  async function runLandAutomationTask(taskId: LandQuickAutomationTask) {
    if (landRushBusy || bulkFertilizeBusyMode || landActionBusyKey || landQuickActionBusy) return;
    setLandQuickActionBusy(taskId);
    setLandError('');
    setLandActionNotice('');
    try {
      const result: any = await RunFarmAutomationTask(taskId);
      if (!result?.ok) {
        throw new Error(result?.message || '快捷操作执行失败');
      }
      setLandActionNotice(result.message || '快捷操作已完成');
      refreshLandDetails();
    } catch (error) {
      setLandError(error instanceof Error ? error.message : String(error));
    } finally {
      setLandQuickActionBusy('');
    }
  }

  async function submitBulkShovel(landIds: number[]) {
    if (landIds.length === 0 || landRushBusy || bulkFertilizeBusyMode || landActionBusyKey || landQuickActionBusy) return;
    setLandQuickActionBusy('shovel');
    setLandError('');
    setLandActionNotice('');
    try {
      const result: any = await FarmShovelLands({ landIds } as any);
      if (result?.ok === false) {
        throw new Error(result.error || '一键铲除提交失败');
      }
      setLandActionNotice(result?.message || `已提交铲除 ${landIds.length} 块有作物地块`);
      refreshLandDetails();
    } catch (error) {
      setLandError(error instanceof Error ? error.message : String(error));
    } finally {
      setLandQuickActionBusy('');
    }
  }

  return (
    <section className="view-stack fill farm-workspace assets-land-view">
      <header className="page-header">
        <div>
          <h1>资产与土地</h1>
          <p>作物收益已接入本地配置；运行时土地、仓库和图鉴操作按独立脚本继续迁移。</p>
        </div>
      </header>

      <div className="assets-land-layout">
        <nav className="asset-tabs" aria-label="资产与土地">
          {tabs.map((tab) => (
            <button
              className={tab.id === activeTab ? 'asset-tab asset-tab-active' : 'asset-tab'}
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
            >
              {tab.icon}
              <span>{tab.label}</span>
            </button>
          ))}
        </nav>

        <div className="asset-panel asset-tab-panel-enter" key={activeTab}>
          {activeTab === 'crops' && (
            <CropAnalyticsPanel payload={cropAnalytics} loading={loading} error={error} />
          )}
          {activeTab === 'lands' && (
            <>
              <LandDetailsPanel
                payload={landDetails}
                loading={landLoading}
                error={landError}
                nowMs={nowMs}
                onRefresh={refreshLandDetails}
                onRush={() => setLandRushOpen(true)}
                onBulkFertilize={submitBulkFertilizer}
                onRunAutomationTask={runLandAutomationTask}
                onBulkShovel={submitBulkShovel}
                bulkFertilizeBusyMode={bulkFertilizeBusyMode}
                landActionBusyKey={landActionBusyKey}
                landQuickActionBusy={Boolean(landQuickActionBusy || landRushBusy)}
                landActionNotice={landActionNotice}
                cardActionsDisabled={landLoading || landDetails?.farmType === 'friend' || Boolean(landQuickActionBusy || landRushBusy || bulkFertilizeBusyMode)}
                onLandCardAction={runLandCardAction}
              />
              <LandRushDialog
                open={landRushOpen}
                lands={landDetails?.lands || []}
                disabled={landDetails?.farmType === 'friend'}
                busy={landRushBusy}
                onClose={() => setLandRushOpen(false)}
                onSubmit={submitLandRush}
              />
            </>
          )}
        {activeTab === 'warehouse' && (
          <WarehousePanel
              payload={warehouse}
              loading={warehouseLoading}
              selling={warehouseSelling}
              error={warehouseError}
              onRefresh={refreshWarehouse}
              onSell={sellWarehouse}
          />
        )}
          {activeTab === 'atlas' && (
            <AtlasPreviewPanel
              payload={atlasPreview}
              loading={atlasLoading}
              error={atlasError}
              activeCategory={activeAtlasCategory}
              purchaseLoading={atlasPurchaseLoading}
              purchaseNotice={atlasPurchaseNotice}
              onCategoryChange={setActiveAtlasCategory}
              onRefresh={refreshAtlasPreview}
              onScanPurchase={scanAtlasLockedCrops}
            />
          )}
        </div>
      </div>
      <WarehouseSellToast summary={warehouseSellToast} />
      <AtlasPurchaseConfirmDialog
        open={atlasPurchaseOpen}
        purchases={atlasPurchasePreview?.plan?.purchases || []}
        busy={atlasPurchaseLoading}
        error={atlasError}
        onCancel={() => {
          setAtlasPurchaseOpen(false);
          setAtlasError('');
        }}
        onConfirm={purchaseAtlasLockedCrops}
      />
    </section>
  );
}

function CropAnalyticsPanel({
  payload,
  loading,
  error,
}: {
  payload?: CropAnalyticsPayloadLike;
  loading: boolean;
  error: string;
}) {
  const [activeStrategy, setActiveStrategy] = useState<CropRankingStrategy>(() => cropRankingStrategyForSort(payload?.sort));
  const items = useMemo(
    () => sortCropRanking(payload?.items || [], activeStrategy),
    [payload?.items, activeStrategy],
  );
  const recommendations = (payload?.recommendations || []).slice(0, 5);
  const activeStrategyLabel = recommendations.find((item) => item.value === activeStrategy)?.label || '经验/小时';
  const levelText = payload?.effectiveMaxLevel
    ? `当前等级 ${payload.effectiveMaxLevel}`
    : '未读取到用户等级';

  return (
    <section className="asset-section">
      <div className="asset-section-header">
        <div>
          <h2>作物分析</h2>
          <p>{levelText}</p>
        </div>
      </div>

      {loading && <div className="asset-empty">正在读取作物配置...</div>}
      {!loading && error && <div className="asset-empty asset-error">{error}</div>}
      {!loading && !error && items.length === 0 && <div className="asset-empty">没有可分析的商店作物。</div>}
      {!loading && !error && items.length > 0 && (
        <div className="asset-data-stack">
          {recommendations.length > 0 && (
            <div className="strategy-grid">
              {recommendations.map((item) => (
                <button
                  aria-pressed={item.value === activeStrategy}
                  className={`strategy-card${item.value === activeStrategy ? ' is-active' : ''}`}
                  key={item.value}
                  onClick={() => setActiveStrategy(item.value as CropRankingStrategy)}
                  type="button"
                >
                  <span>{item.label}</span>
                  <strong>{item.recommended?.name || item.currentRecommended?.name || '-'}</strong>
                  <em>{sourceLabel(item.currentSource)}</em>
                </button>
              ))}
            </div>
          )}
          {payload?.runtimeError && <div className="asset-inline-warning">运行时资料读取失败：{payload.runtimeError}</div>}
          <div className="crop-ranking-heading">
            <strong>{activeStrategyLabel}排行</strong>
            <span>完整列表 {items.length} 项</span>
          </div>
          <div className="asset-table-wrap crop-ranking-wrap">
            <table className="asset-table">
              <thead>
                <tr>
                  <th>作物</th>
                  <th>等级</th>
                  <th>周期</th>
                  <th>经验/小时</th>
                  <th>普肥经验/小时</th>
                  <th>净利润</th>
                  <th>利润/小时</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <strong>{item.name}</strong>
                    </td>
                    <td>{item.level ?? '-'}</td>
                    <td>{item.growTimeText}</td>
                    <td>{formatMetric(item.expPerHour)}</td>
                    <td>{formatMetric(item.normalFertilizerExpPerHour)}</td>
                    <td>{item.netProfit}</td>
                    <td>{formatMetric(item.profitPerHour)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </section>
  );
}

function cropRankingStrategyForSort(sort?: string): CropRankingStrategy {
  switch (sort) {
    case 'level':
      return 'highest_level';
    case 'fert_exp':
      return 'max_fert_exp';
    case 'profit':
      return 'max_profit';
    case 'fert_profit':
      return 'max_fert_profit';
    default:
      return 'max_exp';
  }
}

function sortCropRanking(items: farm.CropAnalyticsItem[], strategy: CropRankingStrategy) {
  return [...items].sort((left, right) => {
    switch (strategy) {
      case 'highest_level':
        return (Number(right.level) || 0) - (Number(left.level) || 0);
      case 'max_fert_exp':
        return (Number(right.normalFertilizerExpPerHour) || 0) - (Number(left.normalFertilizerExpPerHour) || 0);
      case 'max_profit':
        return (Number(right.profitPerHour) || 0) - (Number(left.profitPerHour) || 0);
      case 'max_fert_profit':
        return (Number(right.normalFertilizerProfitPerHour) || 0) - (Number(left.normalFertilizerProfitPerHour) || 0);
      default:
        return (Number(right.expPerHour) || 0) - (Number(left.expPerHour) || 0);
    }
  });
}

export function LandDetailsPanel({
  payload,
  loading,
  error,
  nowMs,
  onRefresh,
  onRush,
  onBulkFertilize,
  onRunAutomationTask,
  onBulkShovel,
  bulkFertilizeBusyMode = '',
  landActionBusyKey = '',
  landQuickActionBusy = false,
  landActionNotice = '',
  cardActionsDisabled = false,
  onLandCardAction,
}: {
  payload?: LandDetailsPayloadLike;
  loading: boolean;
  error: string;
  nowMs: number;
  onRefresh: () => void;
  onRush?: () => void;
  onBulkFertilize?: (mode: LandCardFertilizerMode) => void;
  onRunAutomationTask?: (taskId: LandQuickAutomationTask) => void;
  onBulkShovel?: (landIds: number[]) => void;
  bulkFertilizeBusyMode?: 'normal' | 'organic' | '';
  landActionBusyKey?: string;
  landQuickActionBusy?: boolean;
  landActionNotice?: string;
  cardActionsDisabled?: boolean;
  onLandCardAction?: (action: LandCardAction, land: LandDetailsItemLike, mode?: LandCardFertilizerMode) => void;
}) {
  const sortedLands = useMemo(
    () => [...(payload?.lands || [])].sort((a, b) => (Number(a.landId) || 0) - (Number(b.landId) || 0)),
    [payload?.lands],
  );
  const hasLands = sortedLands.length > 0;
  const [fertilizerTarget, setFertilizerTarget] = useState<LandDetailsItemLike | null>(null);
  const growingLandCount = useMemo(
    () => sortedLands.filter((land) => isGrowingLandForFertilizer(land)).length,
    [sortedLands],
  );
  const panelActions = useMemo(() => {
    const allowed = new Set(['rush', 'fertilize_normal', 'fertilize_organic']);
    return (payload?.actions || [])
      .filter((action) => allowed.has(action.id))
      .map((action) => {
        if (action.id === 'rush') {
          return {
            ...action,
            enabled: Boolean(action.enabled) && !bulkFertilizeBusyMode && !landActionBusyKey && !landQuickActionBusy,
            reason: landQuickActionBusy ? '正在执行其他土地操作' : action.reason,
          };
        }
        const mode = action.id === 'fertilize_organic' ? 'organic' : 'normal';
        const busy = bulkFertilizeBusyMode === mode;
        const otherBusy = Boolean(bulkFertilizeBusyMode) || Boolean(landActionBusyKey) || landQuickActionBusy;
        let enabled = Boolean(action.enabled) && !cardActionsDisabled && !otherBusy && growingLandCount > 0;
        let reason = action.reason || '';
        if (busy) {
          enabled = false;
          reason = '正在施肥...';
        } else if (landQuickActionBusy) {
          enabled = false;
          reason = '正在执行其他土地操作';
        } else if (cardActionsDisabled) {
          enabled = false;
          reason = reason || '当前不可操作';
        } else if (growingLandCount === 0) {
          enabled = false;
          reason = '没有生长中的地块';
        }
        return {
          ...action,
          enabled,
          reason,
          label: busy ? (mode === 'organic' ? '有机施肥中' : '无机施肥中') : action.label,
        };
      });
  }, [bulkFertilizeBusyMode, cardActionsDisabled, growingLandCount, landActionBusyKey, landQuickActionBusy, payload?.actions]);

  function openFertilizerChooser(land: LandDetailsItemLike) {
    if (cardActionsDisabled || !landRushLandId(land) || landActionBusyKey) return;
    setFertilizerTarget(land);
  }

  function selectFertilizer(mode: LandCardFertilizerMode) {
    const target = fertilizerTarget;
    if (!target || cardActionsDisabled || landActionBusyKey) return;
    setFertilizerTarget(null);
    onLandCardAction?.('fertilize', target, mode);
  }

  return (
    <>
      <section className="asset-section land-details-section">
      <div className="asset-section-header">
        <div>
          <h2>土地详情</h2>
          <p>{payload?.message || '土地详情需要迁移 /api/lands 与真实游戏运行时读取命令。'}</p>
        </div>
        <span className="migration-state">{payload?.status || 'not_migrated'}</span>
      </div>

      <div className="land-details-content">
        <div className="land-details-actions">
          <LandQuickActions
            payload={payload}
            lands={sortedLands}
            actions={panelActions}
            busy={landQuickActionBusy || Boolean(landActionBusyKey) || Boolean(bulkFertilizeBusyMode)}
            onAutomationTask={onRunAutomationTask}
            onBulkFertilize={onBulkFertilize}
            onBulkShovel={onBulkShovel}
            onRush={onRush}
          />
        </div>

        <div className="land-details-results">
          {loading && !hasLands && <div className="asset-empty">正在读取土地状态...</div>}
          {!loading && error && !hasLands && <div className="asset-empty asset-error">{error}</div>}
          {error && hasLands && <div className="asset-inline-warning">土地刷新失败：{error}</div>}
          {landActionNotice && <div className="asset-inline-notice" role="status">{landActionNotice}</div>}
          {!loading && !error && !hasLands && (
            <div className="asset-empty">没有土地数据，等待真实运行时读取迁移。</div>
          )}
          {hasLands && (
            <div className="land-grid">
              {sortedLands.map((land) => {
            const landId = landRushLandId(land);
            const shovelBusy = landActionBusyKey === `shovel:${landId}:`;
            const disabled = cardActionsDisabled || !landId || Boolean(landActionBusyKey);
            return (
            <article className={`land-tile land-tile-${land.status || 'unknown'}`} key={land.id}>
              <header className="land-tile-header">
                <span className="land-index">#{land.landId}</span>
                <div className="land-header-badges">
                  {land.needGoldenBug && <span className="land-status-pill land-status-danger">金虫</span>}
                  <span className="land-status-pill">{land.statusLabel}</span>
                </div>
              </header>
              <LandVisual land={land} />
              <div className="land-tile-main">
                <strong>{landDisplayName(land)}</strong>
                <div className="land-pill-row">
                  <span className="land-info-pill land-info-blue land-countdown-full">{landCountdownText(land, nowMs)}</span>
                  <span className="land-info-pill land-info-blue land-countdown-compact">{landCompactCountdownText(land, nowMs)}</span>
                  {seasonLabel(land) && <span className="land-info-pill">{seasonLabel(land)}</span>}
                </div>
              </div>
              <div className="land-tag-list">
                <span className="land-tag land-tag-purple">{land.landTypeLabel || '待识别土地'}</span>
                {mutationLabel(land) && (
                  <span className="land-tag land-tag-mutation">{mutationLabel(land)}</span>
                )}
                {landTags(land).map((tag) => (
                  <span className={tag.tone ? `land-tag land-tag-${tag.tone}` : 'land-tag'} key={tag.label}>
                    {tag.label}
                  </span>
                ))}
              </div>
              <div className="land-card-actions">
                <button
                  type="button"
                  aria-label="选择肥料类型"
                  className="land-card-action land-card-action-fertilize"
                  disabled={disabled}
                  onClick={() => openFertilizerChooser(land)}
                  title="选择肥料类型"
                >
                  <Sprout size={13} />
                  <span className="land-card-action-label">施肥</span>
                </button>
                <button
                  type="button"
                  aria-label="铲除作物"
                  className="land-card-action land-card-action-orange"
                  disabled={disabled}
                  onClick={() => onLandCardAction?.('shovel', land)}
                  title="铲除作物"
                >
                  {shovelBusy ? <Loader2 className="spin" size={13} /> : <Shovel size={13} />}
                  <span className="land-card-action-label">{shovelBusy ? '处理中' : '铲除'}</span>
                </button>
              </div>
            </article>
          );
              })}
            </div>
          )}
        </div>
      </div>
      </section>
      <LandFertilizerChooser
        land={fertilizerTarget}
        busy={Boolean(landActionBusyKey)}
        open={Boolean(fertilizerTarget)}
        onClose={() => setFertilizerTarget(null)}
        onSelect={selectFertilizer}
      />
    </>
  );
}

function LandFertilizerChooser({
  open,
  land,
  busy,
  onClose,
  onSelect,
}: {
  open: boolean;
  land: LandDetailsItemLike | null;
  busy: boolean;
  onClose: () => void;
  onSelect: (mode: LandCardFertilizerMode) => void;
}) {
  if (!open || !land) return null;
  const targetName = landDisplayName(land);
  const title = `对 #${land.landId} ${targetName}施肥`;

  return (
    <div className="land-fertilizer-drawer-backdrop" role="presentation">
      <aside className="land-fertilizer-drawer" role="dialog" aria-modal="true" aria-label={title}>
        <header className="land-fertilizer-header">
          <div>
            <h2>选择肥料类型</h2>
            <p>{title}</p>
          </div>
          <button type="button" aria-label="关闭施肥选择" onClick={onClose}>
            <X size={17} />
          </button>
        </header>
        <div className="land-fertilizer-options">
          <button
            type="button"
            disabled={busy}
            aria-label={`对 #${land.landId} ${targetName}施用无机肥`}
            onClick={() => onSelect('normal')}
          >
            <Sprout size={18} />
            <strong>无机肥</strong>
            <span>立即施用</span>
          </button>
          <button
            type="button"
            disabled={busy}
            aria-label={`对 #${land.landId} ${targetName}施用有机肥`}
            onClick={() => onSelect('organic')}
          >
            <Droplet size={18} />
            <strong>有机肥</strong>
            <span>立即施用</span>
          </button>
        </div>
        <footer>
          <button type="button" disabled={busy} onClick={onClose}>取消</button>
        </footer>
      </aside>
    </div>
  );
}

export function LandRushDialog({
  open,
  lands = [],
  disabled = false,
  busy = false,
  initialSelectionMode = 'auto',
  onClose,
  onSubmit,
}: {
  open: boolean;
  lands: LandDetailsItemLike[];
  disabled?: boolean;
  busy?: boolean;
  initialSelectionMode?: 'auto' | 'manual';
  onClose: () => void;
  onSubmit: (payload: LandRushPayloadLike) => void;
}) {
  const [selectionMode, setSelectionMode] = useState<'auto' | 'manual'>(initialSelectionMode);
  const [thresholdSec, setThresholdSec] = useState(300);
  const [manualLandIds, setManualLandIds] = useState<number[]>([]);
  const [harvestLinkEnabled, setHarvestLinkEnabled] = useState(true);
  const [fertilizerMode, setFertilizerMode] = useState<'normal' | 'organic'>('organic');
  const eligibleLandIds = useMemo(
    () => lands.filter((land) => isLandRushEligible(land, thresholdSec)).map((land) => landRushLandId(land)).filter((id): id is number => !!id),
    [lands, thresholdSec],
  );
  const eligibleLands = useMemo(
    () => lands.filter((land) => isLandRushEligible(land, thresholdSec)),
    [lands, thresholdSec],
  );

  useEffect(() => {
    if (!open) return;
    setSelectionMode(initialSelectionMode);
    setThresholdSec(300);
    setHarvestLinkEnabled(true);
    setFertilizerMode('organic');
  }, [initialSelectionMode, open]);

  useEffect(() => {
    if (!open || selectionMode !== 'auto') return;
    setManualLandIds(eligibleLandIds);
  }, [eligibleLandIds, open, selectionMode]);

  useEffect(() => {
    if (!open || selectionMode !== 'manual') return;
    setManualLandIds((current) => current.filter((id) => eligibleLandIds.includes(id)));
  }, [eligibleLandIds, open, selectionMode]);

  if (!open) return null;

  const selectedLandIds = selectionMode === 'auto' ? eligibleLandIds : manualLandIds;
  const selectedSet = new Set(selectedLandIds);
  const canSubmit = !disabled && !busy && selectedLandIds.length > 0;

  function switchSelectionMode(mode: 'auto' | 'manual') {
    setSelectionMode(mode);
    if (mode === 'manual') {
      setManualLandIds(eligibleLandIds);
    }
  }

  function toggleManualLand(landId: number) {
    setManualLandIds((current) => (
      current.includes(landId) ? current.filter((id) => id !== landId) : [...current, landId]
    ));
  }

  function submit() {
    if (!canSubmit) return;
    onSubmit({
      landIds: selectedLandIds,
      rushThresholdSec: Math.max(0, Number(thresholdSec) || 0),
      harvestLinkEnabled,
      continuousRushEnabled: false,
      fertilizerMode,
    });
  }

  return (
    <div className="land-rush-drawer-backdrop" role="presentation">
      <aside className="land-rush-drawer" role="dialog" aria-modal="true" aria-labelledby="land-rush-title">
        <header className="land-rush-header">
          <div>
            <h2 id="land-rush-title">一键催熟</h2>
            <p>已选 {selectedLandIds.length} 块</p>
          </div>
          <button className="land-rush-icon-button" type="button" aria-label="关闭一键催熟" onClick={onClose}>
            <X size={17} />
          </button>
        </header>

        <div className="land-rush-body">
          <div className="land-rush-segmented" aria-label="地块来源">
            <button className={selectionMode === 'auto' ? 'active' : ''} type="button" onClick={() => switchSelectionMode('auto')}>
              <Filter size={15} />
              <span>自动筛选</span>
            </button>
            <button className={selectionMode === 'manual' ? 'active' : ''} type="button" onClick={() => switchSelectionMode('manual')}>
              <CheckSquare size={15} />
              <span>手动选择</span>
            </button>
          </div>

          <label className="land-rush-field">
            <span>成熟阈值(秒)</span>
            <input
              min={0}
              type="number"
              value={thresholdSec}
              onChange={(event) => setThresholdSec(Number(event.target.value))}
            />
          </label>

          <div className="land-rush-shortcuts">
            {[120, 300, 600].map((seconds) => (
              <button
                className={Number(thresholdSec) === seconds ? 'active' : ''}
                key={seconds}
                type="button"
                onClick={() => setThresholdSec(seconds)}
              >
                {formatRushSeconds(seconds)}
              </button>
            ))}
          </div>

          <div className="land-rush-group">
            <span className="land-rush-label">催熟肥料</span>
            <div className="land-rush-segmented" aria-label="催熟肥料">
              <button className={fertilizerMode === 'normal' ? 'active' : ''} type="button" onClick={() => setFertilizerMode('normal')}>
                无机
              </button>
              <button className={fertilizerMode === 'organic' ? 'active' : ''} type="button" onClick={() => setFertilizerMode('organic')}>
                有机
              </button>
            </div>
          </div>

          <label className="land-rush-check">
            <input
              type="checkbox"
              checked={harvestLinkEnabled}
              onChange={(event) => setHarvestLinkEnabled(event.target.checked)}
            />
            <span>催熟联动收获</span>
          </label>

          {selectionMode === 'manual' ? (
            <div className="land-rush-manual">
              <div className="land-rush-manual-header">
                <strong>可催熟地块</strong>
                <button type="button" onClick={() => setManualLandIds(eligibleLandIds)} disabled={eligibleLandIds.length === 0}>
                  全选
                </button>
              </div>
              {eligibleLands.length > 0 ? (
                eligibleLands.map((land) => {
                  const landId = landRushLandId(land);
                  if (!landId) return null;
                  return (
                    <label className="land-rush-land-row" key={landId}>
                      <input
                        type="checkbox"
                        checked={selectedSet.has(landId)}
                        onChange={() => toggleManualLand(landId)}
                      />
                      <span>#{landId} {landDisplayName(land)} · {formatRushSeconds(Number(land.matureInSec))}后成熟</span>
                    </label>
                  );
                })
              ) : (
                <p className="land-rush-muted">当前阈值内没有可催熟地块。</p>
              )}
            </div>
          ) : (
            <p className="land-rush-muted">自动筛选会选择当前阈值内的生长中地块，已成熟、枯萎和空地会跳过。</p>
          )}
        </div>

        <footer className="land-rush-footer">
          <button className="secondary-action" type="button" onClick={onClose}>取消</button>
          <button className="land-rush-submit" type="button" disabled={!canSubmit} onClick={submit}>
            {busy ? <Loader2 className="spin" size={15} /> : null}
            <span>{busy ? '处理中...' : '催熟联动收获'}</span>
          </button>
        </footer>
      </aside>
    </div>
  );
}

function LandVisual({ land }: { land: LandDetailsItemLike }) {
  const visualClass = land.status === 'empty' ? 'land-visual land-visual-empty' : 'land-visual';
  const imageUrl = land.mutationImageUrl || land.imageUrl;
  const iconUrls = mutationIconUrls(land);
  const mutationIcons = iconUrls.length > 0 && (
    <div className="land-mutation-icons">
      {iconUrls.map((iconUrl, index) => (
        <FallbackImage
          key={`${iconUrl}-${index}`}
          className="land-mutation-icon"
          alt={land.mutationLabel || '变异'}
          src={iconUrl}
        />
      ))}
    </div>
  );
  if (imageUrl) {
    return (
      <div className={visualClass}>
        <LandStageImage alt={landDisplayName(land)} src={imageUrl} />
        {mutationIcons}
      </div>
    );
  }
  return (
    <div aria-label="默认土地图标" className={visualClass} role="img">
      <Sprout size={40} />
      {mutationIcons}
    </div>
  );
}

type LandArtworkBounds = {
  width: number;
  height: number;
  minX: number;
  maxX: number;
  minY: number;
  maxY: number;
};

const visibleArtworkAlphaThreshold = 128;

export function landArtworkVisibleBounds(width: number, height: number, pixels: Uint8ClampedArray): LandArtworkBounds | null {
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0 || pixels.length < width * height * 4) return null;
  let minX = width;
  let maxX = -1;
  let minY = height;
  let maxY = -1;
  for (let y = 0; y < height; y += 1) {
    for (let x = 0; x < width; x += 1) {
      const alpha = pixels[(y * width + x) * 4 + 3];
      if (alpha < visibleArtworkAlphaThreshold) continue;
      minX = Math.min(minX, x);
      maxX = Math.max(maxX, x);
      minY = Math.min(minY, y);
      maxY = Math.max(maxY, y);
    }
  }
  return maxX < 0 ? null : { width, height, minX, maxX, minY, maxY };
}

type LandArtworkCropRect = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export function landArtworkCropRect(bounds: LandArtworkBounds) {
  const { width, height, minX, maxX, minY, maxY } = bounds;
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0 || maxX < minX || maxY < minY) return null;
  const padding = Math.max(2, Math.round(Math.min(width, height) * 0.08));
  const left = Math.max(0, minX - padding);
  const top = Math.max(0, minY - padding);
  const right = Math.min(width - 1, maxX + padding);
  const bottom = Math.min(height - 1, maxY + padding);
  return { x: left, y: top, width: right - left + 1, height: bottom - top + 1 };
}

export function landArtworkDrawPlacement(bounds: LandArtworkBounds, crop: LandArtworkCropRect, size: number) {
  if (!Number.isFinite(size) || size <= 0 || !Number.isFinite(crop.x) || !Number.isFinite(crop.y) || !Number.isFinite(crop.width) || !Number.isFinite(crop.height) || crop.width <= 0 || crop.height <= 0) return null;
  const centerX = (bounds.minX + bounds.maxX) / 2;
  const centerY = (bounds.minY + bounds.maxY) / 2;
  const baseScale = Math.min(size / crop.width, size / crop.height);
  const safeRadius = Math.max(0, size / 2 - Math.max(4, Math.round(size * 0.027)));
  const horizontalReach = Math.max(centerX - bounds.minX, bounds.maxX - centerX);
  const verticalReach = Math.max(centerY - bounds.minY, bounds.maxY - centerY);
  const visibleScale = Math.min(
    horizontalReach > 0 ? safeRadius / horizontalReach : Infinity,
    verticalReach > 0 ? safeRadius / verticalReach : Infinity,
  );
  const scale = Math.min(baseScale, visibleScale);
  const width = crop.width * scale;
  const height = crop.height * scale;
  return {
    x: size / 2 - (centerX - crop.x) * scale,
    y: size / 2 - (centerY - crop.y) * scale,
    width,
    height,
  };
}

function LandStageImage({ alt, src }: { alt: string; src: string }) {
  const [trimmed, setTrimmed] = useState(false);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    setTrimmed(false);
    if (!window.matchMedia('(max-width: 760px)').matches || !document.querySelector('.app-shell-remote')) return;
    let active = true;
    const image = new Image();
    image.onload = () => {
      const canvas = canvasRef.current;
      if (active && canvas && drawTrimmedArtwork(canvas, image)) setTrimmed(true);
    };
    image.src = src;
    return () => {
      active = false;
      image.onload = null;
    };
  }, [src]);

  return (
    <>
      <canvas
        aria-label={alt}
        className="land-visual-image land-visual-canvas"
        hidden={!trimmed}
        ref={canvasRef}
        role="img"
      />
      {!trimmed && <FallbackImage className="land-visual-image" alt={alt} src={src} />}
    </>
  );
}

function drawTrimmedArtwork(canvas: HTMLCanvasElement, image: HTMLImageElement): boolean {
  const bounds = alphaBounds(image);
  const crop = bounds && landArtworkCropRect(bounds);
  if (!bounds || !crop) return false;
  const size = 148;
  const placement = landArtworkDrawPlacement(bounds, crop, size);
  if (!placement) return false;
  canvas.width = size;
  canvas.height = size;
  const context = canvas.getContext('2d');
  if (!context) return false;
  try {
    context.clearRect(0, 0, size, size);
    context.drawImage(image, crop.x, crop.y, crop.width, crop.height, placement.x, placement.y, placement.width, placement.height);
    return true;
  } catch {
    return false;
  }
}

function alphaBounds(image: HTMLImageElement): LandArtworkBounds | null {
  const width = image.naturalWidth;
  const height = image.naturalHeight;
  if (!width || !height) return null;
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext('2d', { willReadFrequently: true });
  if (!context) return null;

  try {
    context.drawImage(image, 0, 0, width, height);
    const pixels = context.getImageData(0, 0, width, height).data;
    return landArtworkVisibleBounds(width, height, pixels);
  } catch {
    return null;
  }
}

function WarehousePanel({
  payload,
  loading,
  selling,
  error,
  onRefresh,
  onSell,
}: {
  payload?: WarehousePayloadLike;
  loading: boolean;
  selling: boolean;
  error: string;
  onRefresh: () => void;
  onSell: (itemKeys: string[]) => void;
}) {
  const [activeCategory, setActiveCategory] = useState('all');
  const [selectedKeys, setSelectedKeys] = useState<string[]>([]);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settingsBusy, setSettingsBusy] = useState(false);
  const [settingsError, setSettingsError] = useState('');
  const [autoSellSettings, setAutoSellSettings] = useState<WarehouseAutoSellSettings>(defaultWarehouseAutoSellSettings);
  const items = (payload?.items || []).filter((item) => !hiddenWarehouseItemNames.has(item.name));
  const categories = mergeWarehouseCategories(payload?.summary?.categoryList, items);
  const filteredItems = activeCategory === 'all'
    ? items
    : items.filter((item) => item.category === activeCategory);
  const sellableFilteredItems = filteredItems.filter((item) => item.canSell && !item.locked);
  const selectedSet = new Set(selectedKeys);
  const selectedSellableKeys = selectedKeys.filter((key) => items.some((item) => item.id === key && item.canSell && !item.locked));
  const allFilteredSelected = sellableFilteredItems.length > 0 && sellableFilteredItems.every((item) => selectedSet.has(item.id));

  function toggleItem(item: WarehouseItemLike) {
    if (!item.canSell || item.locked) return;
    setSelectedKeys((current) => (
      current.includes(item.id) ? current.filter((key) => key !== item.id) : [...current, item.id]
    ));
  }

  function toggleFilteredSelection() {
    const keys = sellableFilteredItems.map((item) => item.id);
    setSelectedKeys((current) => {
      if (allFilteredSelected) {
        return current.filter((key) => !keys.includes(key));
      }
      return Array.from(new Set([...current, ...keys]));
    });
  }

  function submitSell() {
    if (selectedSellableKeys.length === 0 || selling) return;
    onSell(selectedSellableKeys);
    setSelectedKeys([]);
  }

  useEffect(() => {
    LoadWarehouseAutoSellSettings()
      .then((settings) => setAutoSellSettings(settingsToWarehouseAutoSell(settings)))
      .catch((err) => setSettingsError(err instanceof Error ? err.message : String(err)));
  }, []);

  function openAutoSellSettings() {
    setSettingsOpen(true);
    setSettingsBusy(true);
    setSettingsError('');
    LoadWarehouseAutoSellSettings()
      .then((settings) => setAutoSellSettings(settingsToWarehouseAutoSell(settings)))
      .catch((err) => setSettingsError(err instanceof Error ? err.message : String(err)))
      .finally(() => setSettingsBusy(false));
  }

  function saveAutoSellSettings(next: WarehouseAutoSellSettings) {
    setSettingsBusy(true);
    setSettingsError('');
    const payload: storage.WarehouseAutoSellSettings = {
      enabled: next.enabled,
      intervalMinute: Math.max(1, Number(next.intervalMinute) || 1),
      categories: next.categories,
      refreshOnlyOnAutoSell: true,
    };
    SaveWarehouseAutoSellSettings(payload)
      .then((settings) => {
        setAutoSellSettings(settingsToWarehouseAutoSell(settings));
        setSettingsOpen(false);
      })
      .catch((err) => setSettingsError(err instanceof Error ? err.message : String(err)))
      .finally(() => setSettingsBusy(false));
  }

  return (
    <section className="asset-section warehouse-section">
      <div className="asset-section-header">
        <div>
          <h2>仓库</h2>
          <p>{payload?.message || '手动刷新仓库；后台默认仅在出售时刷新仓库快照。'}</p>
        </div>
        <span className="migration-state">{payload?.status || 'not_migrated'}</span>
      </div>

      <div className="warehouse-toolbar">
        <div>
          <strong>仓库分类</strong>
          <span>
            共 {payload?.summary?.totalDistinct || items.length} 类 / {payload?.summary?.totalCount || 0} 个，
            可售 {payload?.summary?.sellableDistinct || 0} 类
          </span>
        </div>
        <div className="warehouse-actions">
          <button className="secondary-action" disabled={loading} type="button" onClick={onRefresh}>
            {loading ? '刷新中...' : '刷新仓库'}
          </button>
          <button className="secondary-action" type="button" onClick={openAutoSellSettings}>
            自动出售设置
          </button>
          <WarehouseSellRecordsButton />
          <button className="secondary-action" disabled={selling || selectedSellableKeys.length === 0} type="button" onClick={submitSell}>
            {selling ? '出售中...' : `出售选中${selectedSellableKeys.length > 0 ? `(${selectedSellableKeys.length})` : ''}`}
          </button>
        </div>
      </div>

      <div className="warehouse-category-tabs" aria-label="仓库分类">
        <button
          className={activeCategory === 'all' ? 'warehouse-category-tab active' : 'warehouse-category-tab'}
          type="button"
          onClick={() => setActiveCategory('all')}
        >
          全部<span>{items.length}</span>
        </button>
        {categories.map((category) => (
          <button
            className={activeCategory === category.key ? 'warehouse-category-tab active' : 'warehouse-category-tab'}
            key={category.key}
            type="button"
            onClick={() => setActiveCategory(category.key)}
          >
            {category.label}<span>{category.distinct}</span>
          </button>
        ))}
      </div>

      {loading && <div className="asset-empty">正在读取仓库状态...</div>}
      {!loading && error && <div className="asset-empty asset-error">{error}</div>}
      {!loading && !error && (payload?.items || []).length === 0 && (
        <div className="asset-empty">没有仓库物品数据，请点击“刷新仓库”读取运行时快照。</div>
      )}
      {!loading && !error && (payload?.items || []).length > 0 && (
        <div className="asset-table-wrap">
          <table className="asset-table warehouse-desktop-table">
            <thead>
              <tr>
                <th>
                  <input
                    aria-label="选择当前分类可售物品"
                    checked={allFilteredSelected}
                    disabled={sellableFilteredItems.length === 0}
                    type="checkbox"
                    onChange={toggleFilteredSelection}
                  />
                </th>
                <th>物品</th>
                <th>分类</th>
                <th>数量</th>
                <th>出售</th>
                <th>估值</th>
              </tr>
            </thead>
            <tbody>
              {filteredItems.map((item) => (
                <tr key={item.id}>
                  <td>
                    <input
                      aria-label={`选择${item.name}`}
                      checked={selectedSet.has(item.id)}
                      disabled={!item.canSell || item.locked}
                      type="checkbox"
                      onChange={() => toggleItem(item)}
                    />
                  </td>
                  <td>
                    <div className="warehouse-item-cell">
                      <strong>{item.name}</strong>
                    </div>
                  </td>
                  <td>{item.categoryLabel}</td>
                  <td>{item.count}</td>
                  <td>{item.canSell ? '可出售' : item.locked ? '锁定' : '不可出售'}</td>
                  <td>{item.estimatedSellPrice > 0 ? `预计 ${item.estimatedSellPrice}` : '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <div className="warehouse-mobile-list">
            {filteredItems.map((item) => (
              <label className="warehouse-mobile-row" key={item.id}>
                <input
                  aria-label={`选择${item.name}`}
                  checked={selectedSet.has(item.id)}
                  disabled={!item.canSell || item.locked}
                  type="checkbox"
                  onChange={() => toggleItem(item)}
                />
                <span className="warehouse-mobile-copy">
                  <strong>{item.name}</strong>
                  <small>{item.categoryLabel} · {item.canSell ? '可出售' : item.locked ? '锁定' : '不可出售'}</small>
                </span>
                <span className="warehouse-mobile-values">
                  <strong>x{item.count}</strong>
                  <em>{item.estimatedSellPrice > 0 ? `预计 ${item.estimatedSellPrice}` : '-'}</em>
                </span>
              </label>
            ))}
          </div>
        </div>
      )}
      <WarehouseAutoSellDialog
        open={settingsOpen}
        busy={settingsBusy}
        error={settingsError}
        settings={autoSellSettings}
        onClose={() => setSettingsOpen(false)}
        onSave={saveAutoSellSettings}
      />
    </section>
  );
}

function WarehouseSellRecordsButton() {
  const [open, setOpen] = useState(false);

  return (
    <>
      <button className="secondary-action" type="button" onClick={() => setOpen(true)}>
        <ReceiptText size={14} />
        出售记录
      </button>
      <WarehouseSellRecordsDialog open={open} onClose={() => setOpen(false)} />
    </>
  );
}

function WarehouseSellRecordsDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [dateKey, setDateKey] = useState('');
  const [records, setRecords] = useState<WarehouseSellRecordLike[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const loadRecords = useCallback((date: string) => {
    setLoading(true);
    setError('');
    FarmWarehouseSellRecords({ date } as any)
      .then((payload) => setRecords(Array.isArray(payload) ? payload as WarehouseSellRecordLike[] : []))
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (!open) return;
    loadRecords(dateKey);
  }, [dateKey, loadRecords, open]);

  if (!open) return null;

  return createPortal(
    <div className="dialog-backdrop warehouse-records-backdrop" role="presentation">
      <aside className="warehouse-records-dialog" role="dialog" aria-modal="true" aria-label="出售记录">
        <header className="warehouse-settings-header">
          <div>
            <h2>出售记录</h2>
          </div>
          <button className="icon-button warehouse-records-close" type="button" aria-label="关闭出售记录" onClick={onClose}>
            <X size={18} />
          </button>
        </header>

        <div className="warehouse-records-filter">
          <label className="warehouse-settings-field">
            <span>筛选日期</span>
            <input type="date" value={dateKey} onChange={(event) => setDateKey(event.target.value)} />
          </label>
          <button className="secondary-action" type="button" onClick={() => loadRecords(dateKey)}>
            {loading ? '读取中...' : '刷新记录'}
          </button>
        </div>

        <div className="warehouse-records-body">
          {error && <div className="settings-message error">{error}</div>}
          {!error && records.length === 0 && (
            <div className="asset-empty">没有符合日期的出售记录。</div>
          )}
          {!error && records.length > 0 && (
            <>
              <div className="asset-table-wrap warehouse-records-table-wrap">
                <table className="asset-table warehouse-records-table warehouse-records-desktop-table">
                  <thead>
                    <tr>
                      <th>时间</th>
                      <th>类型</th>
                      <th>出售数量</th>
                      <th>出售种类</th>
                      <th>出售金额</th>
                      <th>明细</th>
                    </tr>
                  </thead>
                  <tbody>
                    {records.map((record) => (
                      <tr key={record.id || record.occurredAt}>
                        <td>{formatWarehouseRecordTime(record.occurredAt)}</td>
                        <td>{record.mode === 'auto' ? '自动' : '手动'}</td>
                        <td>{record.totalCount || 0}</td>
                        <td>{record.itemKinds || 0}</td>
                        <td>{record.totalAmount || 0}</td>
                        <td>{formatWarehouseRecordItems(record.items)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <div className="warehouse-records-mobile-list">
                {records.map((record) => (
                  <article className="warehouse-record-mobile-row" key={record.id || record.occurredAt}>
                    <strong>{formatWarehouseRecordTime(record.occurredAt)}</strong>
                    <span>{record.mode === 'auto' ? '自动出售' : '手动出售'} · {record.itemKinds || 0} 种</span>
                    <span>数量 {record.totalCount || 0} · 金额 {record.totalAmount || 0}</span>
                    <em>{formatWarehouseRecordItems(record.items)}</em>
                  </article>
                ))}
              </div>
            </>
          )}
        </div>
      </aside>
    </div>,
    document.body,
  );
}

function WarehouseSellToast({ summary }: { summary: WarehouseSellToastSummary | null }) {
  if (!summary) return null;
  return (
    <div className="social-toast ok warehouse-sell-toast" role="status" aria-live="polite">
      <strong>出售完成</strong>
      <span>出售数量 {summary.totalCount}</span>
      <span>出售次数 {summary.times}</span>
      <span>出售金额 {summary.totalAmount}</span>
    </div>
  );
}

function buildWarehouseSellToastSummary(payload: WarehouseSellPayloadLike): WarehouseSellToastSummary {
  const record = payload.sell?.record;
  if (record) {
    return {
      totalCount: Number(record.totalCount) || 0,
      times: 1,
      totalAmount: Number(record.totalAmount) || 0,
    };
  }
  const soldDiff = Array.isArray(payload.sell?.soldDiff) ? payload.sell?.soldDiff as Array<Record<string, unknown>> : [];
  const totalCount = soldDiff.reduce((sum, item) => sum + (Number(item.soldCount || item.count) || 0), 0);
  const totalAmount = soldDiff.reduce((sum, item) => {
    const count = Number(item.soldCount || item.count) || 0;
    const amount = Number(item.amount);
    if (Number.isFinite(amount) && amount > 0) return sum + amount;
    return sum + count * (Number(item.saleUnitPrice || item.unitPrice) || 0);
  }, 0);
  return { totalCount, times: totalCount > 0 ? 1 : 0, totalAmount };
}

function formatWarehouseRecordTime(value?: string) {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

function formatWarehouseRecordItems(items?: WarehouseSellRecordItemLike[]) {
  const list = Array.isArray(items) ? items : [];
  if (list.length === 0) return '-';
  return list.map((item) => `${item.name || item.itemId || '物品'} x${item.count || 0}`).join('，');
}

function WarehouseAutoSellDialog({
  open,
  busy,
  error,
  settings,
  onClose,
  onSave,
}: {
  open: boolean;
  busy: boolean;
  error: string;
  settings: WarehouseAutoSellSettings;
  onClose: () => void;
  onSave: (settings: WarehouseAutoSellSettings) => void;
}) {
  const [draft, setDraft] = useState(settings);

  useEffect(() => {
    if (open) setDraft(settings);
  }, [open, settings]);

  if (!open) return null;

  function toggleCategory(key: string) {
    setDraft((current) => ({
      ...current,
      categories: current.categories.includes(key)
        ? current.categories.filter((category) => category !== key)
        : [...current.categories, key],
    }));
  }

  return (
    <div className="dialog-backdrop" role="presentation">
      <aside className="warehouse-settings-dialog" role="dialog" aria-modal="true" aria-label="自动出售设置">
        <header className="warehouse-settings-header">
          <div>
            <h2>自动出售设置</h2>
            <p>刷新策略固定为仅出售时刷新仓库。</p>
          </div>
          <button className="icon-button" type="button" aria-label="关闭自动出售设置" onClick={onClose}>
            <X size={18} />
          </button>
        </header>

        <div className="warehouse-settings-body">
          <div className="warehouse-settings-group warehouse-settings-execution">
            <strong>执行方式</strong>
            <label className="warehouse-settings-check warehouse-settings-toggle">
              <span>启用自动出售</span>
              <span className="warehouse-settings-toggle-control">
                <input
                  type="checkbox"
                  checked={draft.enabled}
                  onChange={(event) => setDraft((current) => ({ ...current, enabled: event.target.checked }))}
                />
                <span aria-hidden="true" />
              </span>
            </label>

            <label className="warehouse-settings-field">
              <span>出售间隔</span>
              <span className="warehouse-settings-input-unit">
                <input
                  min={1}
                  type="number"
                  value={draft.intervalMinute}
                  onChange={(event) => setDraft((current) => ({ ...current, intervalMinute: Number(event.target.value) }))}
                />
                <em>分钟</em>
              </span>
            </label>
          </div>

          <div className="warehouse-settings-group">
            <strong>自动出售类型</strong>
            <div className="warehouse-settings-options">
              {warehouseCategoryOptions.map((category) => (
                <label className="warehouse-settings-check" key={category.key}>
                  <input
                    type="checkbox"
                    checked={draft.categories.includes(category.key)}
                    onChange={() => toggleCategory(category.key)}
                  />
                  <span>{category.label}</span>
                </label>
              ))}
            </div>
          </div>

          {error && <div className="settings-message error">{error}</div>}
        </div>

        <footer className="warehouse-settings-footer">
          <button className="secondary-action" type="button" onClick={onClose}>取消</button>
          <button className="primary-action" disabled={busy || draft.categories.length === 0} type="button" onClick={() => onSave(draft)}>
            {busy ? '保存中...' : '保存设置'}
          </button>
        </footer>
      </aside>
    </div>
  );
}

function settingsToWarehouseAutoSell(settings: storage.WarehouseAutoSellSettings): WarehouseAutoSellSettings {
  return {
    enabled: Boolean(settings?.enabled),
    intervalMinute: Math.max(1, Number(settings?.intervalMinute) || defaultWarehouseAutoSellSettings.intervalMinute),
    categories: normalizeWarehouseCategoryKeys(settings?.categories),
  };
}

function normalizeWarehouseCategoryKeys(categories?: string[]) {
  const allowed = new Set(warehouseCategoryOptions.map((category) => category.key));
  const next = (categories || []).filter((category) => allowed.has(category));
  return next.length > 0 ? Array.from(new Set(next)) : defaultWarehouseAutoSellSettings.categories;
}

function mergeWarehouseCategories(summaryCategories: WarehouseCategorySummaryLike[] | undefined, items: WarehouseItemLike[]) {
  const fromItems = buildWarehouseCategories(items);
  return warehouseCategoryOptions.map((option) => {
    const summary = summaryCategories?.find((category) => category.key === option.key);
    const fallback = fromItems.find((category) => category.key === option.key);
    return summary || fallback || option;
  });
}

function buildWarehouseCategories(items: WarehouseItemLike[]) {
  const map = new Map<string, WarehouseCategorySummaryLike>();
  items.forEach((item) => {
    const key = warehouseCategoryOptions.some((category) => category.key === item.category) ? item.category : 'tool';
    const current = map.get(key) || { key, label: item.categoryLabel || key, distinct: 0, count: 0 };
    current.distinct += 1;
    current.count += item.count;
    map.set(key, current);
  });
  return Array.from(map.values()).sort((a, b) => b.distinct - a.distinct || a.key.localeCompare(b.key));
}

function ActionGateList({ actions, onAction }: { actions: RuntimeActionGateLike[]; onAction?: (id: string) => void }) {
  if (actions.length === 0) return null;
  return (
    <div className="action-gate-list">
      {actions.map((action) => (
        <button
          className="secondary-action"
          disabled={!action.enabled}
          key={action.id}
          title={action.reason}
          type="button"
          onClick={() => onAction?.(action.id)}
        >
          {action.label}
        </button>
      ))}
    </div>
  );
}

function landRushLandId(land: LandDetailsItemLike) {
  const landId = Number(land.landId || land.id);
  return Number.isFinite(landId) && landId > 0 ? landId : null;
}

function isLandRushEligible(land: LandDetailsItemLike, thresholdSec: number) {
  const landId = landRushLandId(land);
  const matureInSec = Number(land.matureInSec);
  const threshold = Math.max(0, Number(thresholdSec) || 0);
  if (!landId || !Number.isFinite(matureInSec) || matureInSec <= 5 || matureInSec > threshold) return false;
  if (land.status === 'mature' || land.status === 'dead' || land.status === 'empty') return false;
  return Boolean(land.displayPlantName || land.plantName || land.seedId);
}

function isGrowingLandForFertilizer(land: LandDetailsItemLike) {
  const landId = landRushLandId(land);
  if (!landId) return false;
  if (land.canHarvest || land.status === 'mature' || land.status === 'dead' || land.status === 'empty' || land.status === 'locked') {
    return false;
  }
  if (land.status && land.status !== 'growing' && land.status !== 'unknown') {
    return false;
  }
  return Boolean(land.displayPlantName || land.plantName || land.seedId || land.status === 'growing');
}

export function fertilizerTargetLandIds(lands: LandDetailsItemLike[]) {
  const ids = lands
    .filter((land) => isGrowingLandForFertilizer(land))
    .map((land) => {
      const anchorId = Number(land.occupancyAnchorLandId);
      return Number.isFinite(anchorId) && anchorId > 0 ? anchorId : landRushLandId(land);
    })
    .filter((id): id is number => !!id);
  return [...new Set(ids)];
}

function LandQuickActions({
  payload,
  lands,
  actions,
  busy,
  onAutomationTask,
  onRush,
  onBulkFertilize,
  onBulkShovel,
}: {
  payload?: LandDetailsPayloadLike;
  lands: LandDetailsItemLike[];
  actions: RuntimeActionGateLike[];
  busy: boolean;
  onAutomationTask?: (taskId: LandQuickAutomationTask) => void;
  onRush?: () => void;
  onBulkFertilize?: (mode: LandCardFertilizerMode) => void;
  onBulkShovel?: (landIds: number[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [shovelConfirmLandIds, setShovelConfirmLandIds] = useState<number[]>([]);
  const ownFarmReady = payload?.status === 'runtime' && payload.farmType === 'own';
  const ownFarmReason = payload?.status !== 'runtime'
    ? '游戏运行时未就绪'
    : payload?.farmType !== 'own'
      ? '仅支持自家农场'
      : '';
  const shovelLandIds = bulkShovelTargetLandIds(lands);
  const gateByID = new Map(actions.map((action) => [action.id, action]));
  const blockedReason = busy ? '正在执行其他土地操作' : '';
  const actionGate = (id: string) => gateByID.get(id) || { id, label: id, enabled: false, reason: '当前不可操作' };
  const automationAction = (id: LandQuickAutomationTask, label: string, icon: ReactNode) => ({
    id,
    label,
    icon,
    enabled: ownFarmReady && !busy,
    reason: blockedReason || ownFarmReason,
    onSelect: () => onAutomationTask?.(id),
  });
  const rushGate = actionGate('rush');
  const normalGate = actionGate('fertilize_normal');
  const organicGate = actionGate('fertilize_organic');
  const commandItems: LandQuickActionMenuItem[] = [
    automationAction('own_base', '一键务农', <Sprout size={15} />),
    automationAction('own_collect', '一键收获', <CheckSquare size={15} />),
    automationAction('own_plant', '一键种植', <Sprout size={15} />),
    {
      id: 'shovel',
      label: '一键铲除',
      icon: <Shovel size={15} />,
      danger: true,
      enabled: ownFarmReady && !busy && shovelLandIds.length > 0,
      reason: blockedReason || ownFarmReason || '没有有作物的地块',
      onSelect: () => setShovelConfirmLandIds(shovelLandIds),
    },
    {
      id: 'rush',
      label: '一键催熟',
      icon: <Sprout size={15} />,
      enabled: Boolean(rushGate.enabled) && !busy,
      reason: blockedReason || rushGate.reason,
      onSelect: () => onRush?.(),
    },
    {
      id: 'fertilize_normal',
      label: '一键无机肥',
      icon: <Sprout size={15} />,
      enabled: Boolean(normalGate.enabled) && !busy,
      reason: blockedReason || normalGate.reason,
      onSelect: () => onBulkFertilize?.('normal'),
    },
    {
      id: 'fertilize_organic',
      label: '一键有机肥',
      icon: <Droplet size={15} />,
      enabled: Boolean(organicGate.enabled) && !busy,
      reason: blockedReason || organicGate.reason,
      onSelect: () => onBulkFertilize?.('organic'),
    },
  ];

  useEffect(() => {
    if (!open && shovelConfirmLandIds.length === 0) return;
    if (typeof document === 'undefined') return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      if (shovelConfirmLandIds.length > 0) setShovelConfirmLandIds([]);
      else setOpen(false);
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [open, shovelConfirmLandIds.length]);

  function selectCommand(item: { id: string; enabled: boolean; onSelect: () => void }) {
    if (!item.enabled) return;
    setOpen(false);
    item.onSelect();
  }

  return (
    <div className="land-quick-actions">
      <button
        className="secondary-action land-quick-trigger"
        type="button"
        aria-label="打开快捷操作"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls="land-quick-actions-menu"
        onClick={() => setOpen((value) => !value)}
      >
        快捷操作 <ChevronDown size={14} />
      </button>
      {open && (
        <>
          <div className="land-quick-menu-backdrop" role="presentation" onClick={() => setOpen(false)} />
          <div className="land-quick-menu" id="land-quick-actions-menu" role="menu" aria-label="快捷操作菜单">
            <header className="land-quick-menu-heading">
              <strong>快捷操作</strong>
              <button type="button" aria-label="关闭快捷操作" onClick={() => setOpen(false)}><X size={16} /></button>
            </header>
            {commandItems.map((item, index) => (
              <div key={item.id}>
                {index === 4 && <div className="land-quick-menu-divider" role="separator" />}
                <button
                  className={item.danger ? 'land-quick-menu-item danger' : 'land-quick-menu-item'}
                  type="button"
                  role="menuitem"
                  aria-label={item.label}
                  disabled={!item.enabled}
                  onClick={() => selectCommand(item)}
                >
                  {item.icon}
                  <span>{item.label}</span>
                  {item.reason && <small>{item.reason}</small>}
                </button>
              </div>
            ))}
          </div>
        </>
      )}
      <LandBulkShovelConfirmDialog
        landIds={shovelConfirmLandIds}
        onCancel={() => setShovelConfirmLandIds([])}
        onConfirm={() => {
          onBulkShovel?.(shovelConfirmLandIds);
          setShovelConfirmLandIds([]);
        }}
      />
    </div>
  );
}

function LandBulkShovelConfirmDialog({
  landIds,
  onCancel,
  onConfirm,
}: {
  landIds: number[];
  onCancel: () => void;
  onConfirm: () => void;
}) {
  if (landIds.length === 0) return null;
  return (
    <div className="land-quick-confirm-backdrop" role="presentation">
      <section className="land-quick-confirm" role="alertdialog" aria-modal="true" aria-label="确认一键铲除">
        <header>
          <h2>确认一键铲除</h2>
          <button type="button" aria-label="关闭一键铲除确认" onClick={onCancel}><X size={17} /></button>
        </header>
        <p>即将铲除 {landIds.length} 块有作物地块，此操作不会收获或恢复作物。</p>
        <footer>
          <button type="button" onClick={onCancel}>取消</button>
          <button type="button" aria-label={`确认铲除 ${landIds.length} 块地`} onClick={onConfirm}>确认铲除</button>
        </footer>
      </section>
    </div>
  );
}

export function bulkShovelTargetLandIds(lands: LandDetailsItemLike[]) {
  const ids = lands
    .filter((land) => {
      if (!landRushLandId(land) || land.status === 'empty' || land.status === 'locked') return false;
      return Boolean(land.displayPlantName || land.plantName || land.seedId);
    })
    .map((land) => {
      const anchorId = Number(land.occupancyAnchorLandId);
      return Number.isFinite(anchorId) && anchorId > 0 ? anchorId : landRushLandId(land);
    })
    .filter((id): id is number => !!id);
  return [...new Set(ids)];
}

function formatRushSeconds(value: number) {
  const seconds = Math.max(0, Math.ceil(Number(value) || 0));
  if (seconds < 60) return `${seconds}秒`;
  const minutes = Math.floor(seconds / 60);
  const rest = seconds % 60;
  return rest > 0 ? `${minutes}分${rest}秒` : `${minutes}分钟`;
}

export function AtlasPurchaseConfirmDialog({
  open,
  purchases,
  busy,
  error,
  onCancel,
  onConfirm,
}: {
  open: boolean;
  purchases: AtlasPurchaseItemLike[];
  busy: boolean;
  error: string;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  if (!open) return null;

  const totalCount = purchases.reduce((sum, item) => sum + item.count, 0);
  return (
    <div className="dialog-backdrop atlas-purchase-dialog-backdrop" role="presentation">
      <section className="atlas-purchase-dialog" role="dialog" aria-modal="true" aria-label="确认购买未解锁种子">
        <header className="atlas-purchase-header">
          <div>
            <h2>确认购买未解锁种子</h2>
            <p>已扫描出 {purchases.length} 种种子，共 {totalCount} 个。</p>
          </div>
          <button className="icon-button light" type="button" aria-label="关闭确认购买" onClick={onCancel}>
            <X size={18} />
          </button>
        </header>
        <div className="atlas-purchase-body">
          <ul>
            {purchases.map((item) => (
              <li key={`${item.seedId}-${item.goodsId}`}>
                <span>{item.seedName}</span>
                <strong>x{item.count}</strong>
              </li>
            ))}
          </ul>
          {error && <p className="atlas-purchase-dialog-error" role="alert">{error}</p>}
        </div>
        <footer>
          <button name="cancel-atlas-purchase" type="button" disabled={busy} onClick={onCancel}>取消</button>
          <button name="confirm-atlas-purchase" className="primary-button" type="button" disabled={busy} onClick={onConfirm}>
            {busy ? '购买中...' : '确认购买'}
          </button>
        </footer>
      </section>
    </div>
  );
}

function AtlasPreviewPanel({
  payload,
  loading,
  error,
  activeCategory,
  purchaseLoading,
  purchaseNotice,
  onCategoryChange,
  onRefresh,
  onScanPurchase,
}: {
  payload?: AtlasPreviewPayloadLike;
  loading: boolean;
  error: string;
  activeCategory: AtlasCategory;
  purchaseLoading: boolean;
  purchaseNotice: string;
  onCategoryChange: (category: AtlasCategory) => void;
  onRefresh: () => void;
  onScanPurchase: () => void;
}) {
  const sections = payload?.sections || [];
  const cropSection = sections.find((section) => section.id === 'crop') || { id: 'crop', label: '作物图鉴', items: [], total: 0 };
  const mutationSection = sections.find((section) => section.id === 'mutation') || { id: 'mutation', label: '超变图鉴', items: [], total: 0 };
  const activeSection = activeCategory === 'mutation' ? mutationSection : cropSection;
  const items = activeSection.items || [];
  const summary = activeSection.summary || summarizeAtlasSection(items, activeSection.total);
  const showPurchaseActions = activeCategory === 'crop';
  const statusText = payload?.status === 'runtime' ? '运行时数据' : '静态预览';

  return (
    <section className="asset-section atlas-section">
      <div className="atlas-toolbar">
        <div className="atlas-toolbar-copy">
          <h2>图鉴</h2>
          <p>{payload?.message || '图鉴已从游戏运行时刷新。'}</p>
        </div>
        <div className="atlas-toolbar-actions">
          <span className="migration-state migration-state-done">{statusText}</span>
          <button className="secondary-action" type="button" disabled={loading || !payload?.refreshEnabled} onClick={onRefresh}>
            刷新图鉴
          </button>
          {showPurchaseActions && (
            <button name="scan-atlas-locked-seeds" className="secondary-action atlas-buy-action" type="button" disabled={purchaseLoading || !payload?.buyEnabled || items.length === 0} onClick={onScanPurchase}>
              {purchaseLoading ? '扫描中...' : '一键购买未解锁种子'}
            </button>
          )}
        </div>
      </div>

      {loading && <div className="asset-empty">正在读取图鉴配置...</div>}
      {!loading && error && <div className="asset-empty asset-error">{error}</div>}
      {!loading && !error && (
        <div className="atlas-preview-grid">
          <div className="atlas-category-tabs" role="tablist" aria-label="图鉴分类">
            <button className={activeCategory === 'crop' ? 'active' : ''} type="button" onClick={() => onCategoryChange('crop')}>
              作物图鉴
              <span>{cropSection.total || cropSection.items?.length || 0}</span>
            </button>
            <button className={activeCategory === 'mutation' ? 'active' : ''} type="button" onClick={() => onCategoryChange('mutation')}>
              超变图鉴
              <span>{mutationSection.total || mutationSection.items?.length || 0}</span>
            </button>
          </div>

          <div className="atlas-summary-strip">
            <div>
              <span>当前分类</span>
              <strong>{activeSection.label}</strong>
            </div>
            <div>
              <span>总数</span>
              <strong>{summary.total}</strong>
            </div>
            <div>
              <span>已解锁</span>
              <strong>{summary.unlocked}</strong>
            </div>
            <div>
              <span>未解锁</span>
              <strong>{summary.locked}</strong>
            </div>
          </div>

          {showPurchaseActions && purchaseNotice && <div className="asset-empty">{purchaseNotice}</div>}

          {items.length === 0 && <div className="asset-empty">没有可预览的图鉴条目。</div>}
          {items.length > 0 && (
            <div className="atlas-card-grid">
              {items.map((item) => {
                const cardTypeClass = activeCategory === 'mutation' ? 'atlas-card-mutation' : 'atlas-card-crop';
                const groupLabel = activeCategory === 'mutation' ? mutationAtlasGroupLabel(item) : '作物';
                return (
                  <article className={`atlas-card ${cardTypeClass} ${item.locked ? 'atlas-card-locked' : ''}`} key={`${activeCategory}-${item.id || item.fruitId || item.name}`}>
                    <div className="atlas-card-art">
                      {item.imageUrl ? <FallbackImage alt="" src={item.imageUrl} /> : <span>{atlasInitial(item.name)}</span>}
                    </div>
                    <div className="atlas-card-body">
                      <strong>{item.name || `图鉴 ${item.id}`}</strong>
                      <div className="atlas-card-pills">
                        <span className={item.locked ? 'atlas-pill atlas-pill-locked' : 'atlas-pill atlas-pill-unlocked'}>{item.locked ? '未解锁' : '已解锁'}</span>
                        <span className="atlas-pill atlas-pill-type">{groupLabel}</span>
                      </div>
                    </div>
                  </article>
                );
              })}
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function summarizeAtlasSection(items: AtlasItemLike[], total?: number) {
  return {
    total: total || items.length,
    unlocked: items.filter((item) => item.unlocked || !item.locked).length,
    locked: items.filter((item) => item.locked).length,
  };
}

function atlasInitial(name: string) {
  const trimmed = String(name || '').trim();
  return trimmed ? trimmed.slice(0, 1) : '?';
}

function mutationAtlasGroupLabel(item: AtlasItemLike) {
  if (item.groupName) return item.groupName;
  if (item.name?.startsWith('黄金·')) return '黄金果实';
  if (item.name?.startsWith('装扮·')) return '装扮果实';
  return '超变';
}

function formatMetric(value: number) {
  if (!Number.isFinite(value)) return '-';
  return Number.isInteger(value) ? String(value) : value.toFixed(2);
}

function landDisplayName(land: LandDetailsItemLike) {
  return land.displayPlantName || land.plantName || (land.status === 'empty' ? '空地' : `土地 ${land.landId}`);
}

function mutationLabel(land: LandDetailsItemLike) {
  const label = land.mutationLabel || land.mutationTypes?.map((item) => item.name).filter(Boolean).join('、') || '';
  return land.hasMutation && label ? `变异·${label}` : '';
}

function mutationIconUrls(land: LandDetailsItemLike) {
  const typeIconUrls = land.mutationTypes
    ?.map((item) => item.iconUrl)
    .filter((iconUrl): iconUrl is string => Boolean(iconUrl)) || [];
  return typeIconUrls.length > 0 ? typeIconUrls : land.mutationIconUrl ? [land.mutationIconUrl] : [];
}

function landCountdownText(land: LandDetailsItemLike, nowMs: number) {
  if (land.status === 'empty') return land.matureEtaText || '空地';
  if (land.status === 'locked') return land.matureEtaText || '未解锁';
  if (land.status === 'mature' || land.canHarvest) return '已成熟';
  const matureAtMs = Number(land.matureAtMs) || 0;
  if (matureAtMs > 0) {
    const seconds = Math.max(0, Math.ceil((matureAtMs - nowMs) / 1000));
    return seconds <= 0 ? '已成熟' : `预计 ${formatCountdown(seconds)} 后成熟`;
  }
  const matureInSec = Number(land.matureInSec);
  if (Number.isFinite(matureInSec) && matureInSec > 0) {
    return `预计 ${formatCountdown(matureInSec)} 后成熟`;
  }
  return land.matureEtaText || land.statusLabel;
}

export function landCompactCountdownText(land: LandDetailsItemLike, nowMs: number) {
  if (land.status === 'empty') return '空地';
  if (land.status === 'locked') return '未解锁';
  if (land.status === 'mature' || land.canHarvest) return '可收';
  const matureAtMs = Number(land.matureAtMs) || 0;
  if (matureAtMs > 0) {
    const seconds = Math.max(0, Math.ceil((matureAtMs - nowMs) / 1000));
    return seconds <= 0 ? '可收' : formatCompactCountdown(seconds);
  }
  const matureInSec = Number(land.matureInSec);
  if (Number.isFinite(matureInSec) && matureInSec > 0) return formatCompactCountdown(matureInSec);
  return land.statusLabel;
}

function formatCountdown(seconds: number) {
  const total = Math.max(0, Math.floor(seconds));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
}

function formatCompactCountdown(seconds: number) {
  const total = Math.max(0, Math.floor(seconds));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, '0')}`
    : `${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
}

function landTags(land: LandDetailsItemLike) {
  const tags: Array<{ label: string; tone?: 'good' | 'warn' | 'danger' }> = [];
  const footprint = formatLandFootprint(land.occupancyPlantSize || land.landSize || 0);
  if (footprint) {
    tags.push({ label: footprint });
  }
  if (land.canHarvest) tags.push({ label: '可收获', tone: 'good' });
  if (land.needWater) tags.push({ label: '浇水', tone: 'warn' });
  if (land.needWeed) tags.push({ label: '除草', tone: 'warn' });
  if (land.needBug) tags.push({ label: '杀虫', tone: 'warn' });
  if (land.needGoldenBug) tags.push({ label: '金虫', tone: 'danger' });
  if (land.needEraseDead) tags.push({ label: '铲枯萎', tone: 'danger' });
  return tags.slice(0, 6);
}

function seasonLabel(land: LandDetailsItemLike) {
  const totalSeason = Number(land.totalSeason);
  if (!Number.isFinite(totalSeason) || totalSeason <= 0) return '';
  const currentSeason = Number(land.currentSeason);
  return `第 ${currentSeason > 0 ? currentSeason : 1}/${totalSeason} 季`;
}

function formatLandFootprint(size: number) {
  if (!Number.isFinite(size) || size <= 1) return '';
  return `${size}*${size}占地`;
}

function sourceLabel(source?: string) {
  if (source === 'backpack') return '背包可用';
  if (source === 'shop') return '商店可买';
  if (source === 'static') return '理论推荐';
  return source || '推荐';
}
