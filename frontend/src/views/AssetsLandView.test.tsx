import { renderToStaticMarkup } from 'react-dom/server';
import { act, create } from 'react-test-renderer';
// @ts-expect-error The frontend tsconfig intentionally omits Node types; this test reads CSS text only.
import { readFileSync } from 'node:fs';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const atlasActions = vi.hoisted(() => ({
  preview: vi.fn(),
  purchase: vi.fn(),
  refresh: vi.fn(),
}));

const landActions = vi.hoisted(() => ({
  runAutomationTask: vi.fn(),
  rush: vi.fn(),
  shovel: vi.fn(),
}));

vi.mock('../../wailsjs/go/desktop/App', () => ({
  FarmAtlasBuyLockedPreview: atlasActions.preview,
  FarmAtlasBuyLockedCrops: atlasActions.purchase,
  FarmAtlasPreview: atlasActions.refresh,
  FarmCropAnalytics: vi.fn(),
  FarmFertilizeLand: vi.fn(),
  FarmLandRush: landActions.rush,
  FarmShovelLands: landActions.shovel,
  FarmWarehouseRefresh: vi.fn(),
  FarmWarehouseSell: vi.fn(),
  FarmWarehouseSellRecords: vi.fn(),
  SaveWarehouseAutoSellSettings: vi.fn(),
  RunFarmAutomationTask: landActions.runAutomationTask,
  WarehouseAutoSellSettings: vi.fn(),
}));

import * as AssetsLandModule from './AssetsLandView';
import { AssetsLandView, AtlasPurchaseConfirmDialog, bulkShovelTargetLandIds, fertilizerTargetLandIds, LandDetailsPanel, LandRushDialog } from './AssetsLandView';

describe('AssetsLandView', () => {
  beforeEach(() => {
    atlasActions.preview.mockReset();
    atlasActions.purchase.mockReset();
    atlasActions.refresh.mockReset();
    landActions.runAutomationTask.mockReset();
    landActions.rush.mockReset();
    landActions.shovel.mockReset();
  });

  it('marks the active assets panel as an animated tab surface', () => {
    const html = renderToStaticMarkup(<AssetsLandView />);

    expect(html).toContain('asset-tab-panel-enter');
  });

  it('renders config-backed crop analytics instead of a placeholder card', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialCropAnalytics={{
          source: 'resources/gameConfig',
          sort: 'exp',
          requestedMaxLevel: 0,
          effectiveMaxLevel: 2,
          levelSource: 'profile',
          strategies: [],
          recommendations: [
            {
              value: 'max_exp',
              label: '经验/小时最高',
              currentSource: 'backpack',
              recommended: {
                id: 1020002,
                seedId: 20002,
                name: '白萝卜',
                seasons: 1,
                level: 1,
                growTime: 60,
                growTimeText: '1分',
                reduceSecApplied: 30,
                harvestExp: 1,
                expPerHour: 60,
                normalFertilizerExpPerHour: 120,
                income: 10,
                netProfit: 9,
                profitPerHour: 540,
                normalFertilizerProfitPerHour: 1080,
                fruitId: 40002,
                fruitCount: 5,
                fruitPrice: 2,
                seedPrice: 1,
                plantSize: 1,
                shopEligible: true,
              },
            },
          ],
          items: [
            {
              id: 1020002,
              seedId: 20002,
              name: '白萝卜',
              seasons: 1,
              level: 1,
              growTime: 60,
              growTimeText: '1分',
              reduceSecApplied: 30,
              harvestExp: 1,
              expPerHour: 60,
              normalFertilizerExpPerHour: 120,
              income: 10,
              netProfit: 9,
              profitPerHour: 540,
              normalFertilizerProfitPerHour: 1080,
              fruitId: 40002,
              fruitCount: 5,
              fruitPrice: 2,
              seedPrice: 1,
              plantSize: 1,
              shopEligible: true,
            },
          ],
        }}
      />,
    );

    expect(html).toContain('作物分析');
    expect(html).toContain('当前等级 2');
    expect(html).toContain('经验/小时最高');
    expect(html).toContain('白萝卜');
    expect(html).toContain('经验/小时');
    expect(html).toContain('60');
    expect(html).not.toContain('按经验/小时排序');
    expect(html).not.toContain('数据来自 resources/gameConfig');
    expect(html).not.toContain('已迁移');
    expect(html).not.toContain('种子 20002');
    expect(html).not.toContain('待迁移收益、经验和背包种子策略分析');
  });

  it('switches the full crop ranking when a strategy card is selected', () => {
    const crop = (id: number, name: string, level: number, expPerHour: number) => ({
      id,
      seedId: 20000 + id,
      name,
      seasons: 1,
      level,
      growTime: 3600,
      growTimeText: '1时',
      reduceSecApplied: 600,
      harvestExp: expPerHour,
      expPerHour,
      normalFertilizerExpPerHour: expPerHour,
      income: 100,
      netProfit: 90,
      profitPerHour: expPerHour,
      normalFertilizerProfitPerHour: expPerHour,
      fruitId: 40000 + id,
      fruitCount: 1,
      fruitPrice: 100,
      seedPrice: 10,
      plantSize: 1,
      shopEligible: true,
    });
    const crops = Array.from({ length: 13 }, (_, index) => crop(index + 1, `作物${index + 1}`, index + 1, 100 - index));
    crops[12] = crop(13, '最高等级作物', 99, 1);
    const renderer = create(
      <AssetsLandView
        initialCropAnalytics={{
          items: crops,
          recommendations: [
            { value: 'highest_level', label: '最高等级作物', recommended: crops[12] },
            { value: 'max_exp', label: '经验/小时最高', recommended: crops[0] },
          ],
        }}
        initialAtlasPreview={{ sections: [] }}
        initialLandDetails={{ lands: [] }}
        initialWarehouse={{ items: [] }}
      />,
    );

    const strategyButtons = renderer.root.findAllByType('button').filter((node) => String(node.props.className || '').includes('strategy-card'));
    expect(strategyButtons).toHaveLength(2);
    expect(renderer.root.findAllByType('tbody')[0].findAllByType('tr')).toHaveLength(13);

    act(() => {
      strategyButtons[0].props.onClick();
    });

    const firstCrop = renderer.root.findAllByType('tbody')[0].findAllByType('tr')[0].findByType('strong');
    expect(firstCrop.children.join('')).toBe('最高等级作物');
    expect(renderer.root.findAllByType('button').find((node) => node.props['aria-pressed'] === true)?.props.className).toContain('is-active');
  });

  it('renders atlas static preview with runtime controls disabled', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="atlas"
        initialAtlasPreview={{
          source: 'resources/gameConfig',
          status: 'static_preview',
          message: '图鉴运行时刷新和购买待迁移，当前展示本地作物配置预览。',
          refreshEnabled: false,
          buyEnabled: false,
          sections: [
            {
              id: 'crop',
              label: '作物图鉴',
              total: 1,
              items: [
                {
                  id: 1020002,
                  name: '白萝卜',
                  seedId: 20002,
                  fruitId: 40002,
                  level: 1,
                  seasons: 1,
                  growTime: 60,
                },
              ],
            },
            {
              id: 'mutation',
              label: '超变图鉴',
              total: 1,
              items: [
                {
                  id: 49001,
                  name: '黄金·白萝卜',
                  seedId: 0,
                  fruitId: 49001,
                  level: 0,
                  seasons: 1,
                  growTime: 0,
                  progress: 40,
                  atlasType: 'mutation',
                },
              ],
            },
          ],
        }}
      />,
    );

    expect(html).toContain('图鉴');
    expect(html).toContain('作物图鉴');
    expect(html).toContain('白萝卜');
    expect(html).toContain('atlas-toolbar');
    expect(html).toContain('atlas-card-grid');
    expect(html).toContain('作物');
    expect(html).toContain('刷新图鉴');
    expect(html).toContain('一键购买未解锁种子');
    expect(html).not.toContain('解锁购买预览');
    expect(html).not.toContain('确认购买');
    expect(html).not.toContain('购买预览');
    expect(html).toContain('超变图鉴');
    expect(html).toContain('disabled=""');
    expect(html).not.toContain('等级 1');
    expect(html).not.toContain('种子 20002');
  });

  it('hides locked seed purchase controls on the mutation atlas category', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="atlas"
        initialAtlasCategory="mutation"
        initialAtlasPreview={{
          source: 'runtime',
          status: 'runtime',
          message: '图鉴已从游戏运行时刷新。',
          refreshEnabled: true,
          buyEnabled: true,
          sections: [
            {
              id: 'crop',
              label: '作物图鉴',
              total: 1,
              items: [
                {
                  id: 1020002,
                  name: '白萝卜',
                  seedId: 20002,
                  fruitId: 40002,
                  level: 1,
                  seasons: 1,
                  growTime: 60,
                  locked: true,
                  atlasType: 'crop',
                },
              ],
            },
            {
              id: 'mutation',
              label: '超变图鉴',
              total: 1,
              items: [
                {
                  id: 49001,
                  name: '黄金·白萝卜',
                  seedId: 0,
                  fruitId: 49001,
                  level: 0,
                  seasons: 1,
                  growTime: 0,
                  progress: 40,
                  groupName: '黄金果实',
                  unlocked: true,
                  atlasType: 'mutation',
                },
              ],
            },
          ],
        }}
      />,
    );

    expect(html).toContain('超变图鉴');
    expect(html).toContain('黄金·白萝卜');
    expect(html).toContain('黄金果实');
    expect(html).not.toContain('解锁购买预览');
    expect(html).not.toContain('确认购买');
    expect(html).not.toContain('图鉴点数 40');
    expect(html).not.toContain('果实 49001');
  });

  it('scans before opening a confirm dialog and only purchases after confirmation', async () => {
    atlasActions.preview.mockResolvedValue({
      ok: true,
      plan: {
        purchases: [
          { seedId: 20060, seedName: '莲藕', goodsId: 1, price: 1, count: 1, requiredLevel: 1 },
          { seedId: 20061, seedName: '红玫瑰', goodsId: 2, price: 1, count: 2, requiredLevel: 1 },
        ],
        summary: { purchasable: 2, skipped: 0 },
      },
    });
    atlasActions.purchase.mockResolvedValue({ ok: true, plan: { purchases: [] } });

    const renderer = create(
      <AssetsLandView
        initialTab="atlas"
        initialCropAnalytics={{ items: [] }}
        initialLandDetails={{ lands: [] }}
        initialAtlasPreview={{
          status: 'runtime',
          refreshEnabled: true,
          buyEnabled: true,
          sections: [{
            id: 'crop',
            label: '作物图鉴',
            total: 1,
            items: [{ id: 1020060, name: '莲藕', seedId: 20060, fruitId: 40060, level: 1, seasons: 1, growTime: 60, locked: true }],
          }],
        }}
      />,
    );

    await act(async () => {
      await renderer.root.findByProps({ name: 'scan-atlas-locked-seeds' }).props.onClick();
    });

    const dialog = renderer.root.findByProps({ 'aria-label': '确认购买未解锁种子' });
    const purchaseRows = dialog.findAllByType('li').map((row) => `${row.findByType('span').children.join('')}${row.findByType('strong').children.join('')}`);
    expect(purchaseRows).toContain('莲藕x1');
    expect(purchaseRows).toContain('红玫瑰x2');

    act(() => {
      renderer.root.findByProps({ name: 'cancel-atlas-purchase' }).props.onClick();
    });
    expect(atlasActions.purchase).not.toHaveBeenCalled();

    atlasActions.refresh.mockResolvedValue({ sections: [] });
    await act(async () => {
      renderer.root.findByProps({ name: 'scan-atlas-locked-seeds' }).props.onClick();
      await Promise.resolve();
    });
    await act(async () => {
      renderer.root.findByProps({ name: 'confirm-atlas-purchase' }).props.onClick();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(atlasActions.purchase).toHaveBeenCalledOnce();
    expect(renderer.root.findAllByProps({ 'aria-label': '确认购买未解锁种子' })).toHaveLength(0);
  });

  it('shows a no-purchase message instead of opening a dialog for an empty plan', async () => {
    atlasActions.preview.mockResolvedValue({ ok: true, plan: { purchases: [], summary: { purchasable: 0, skipped: 1 } } });
    const renderer = create(
      <AssetsLandView
        initialTab="atlas"
        initialCropAnalytics={{ items: [] }}
        initialLandDetails={{ lands: [] }}
        initialAtlasPreview={{
          status: 'runtime',
          refreshEnabled: true,
          buyEnabled: true,
          sections: [{
            id: 'crop',
            label: '作物图鉴',
            total: 1,
            items: [{ id: 1020060, name: '莲藕', seedId: 20060, fruitId: 40060, level: 1, seasons: 1, growTime: 60, locked: true }],
          }],
        }}
      />,
    );

    await act(async () => {
      await renderer.root.findByProps({ name: 'scan-atlas-locked-seeds' }).props.onClick();
    });

    expect(renderer.root.findAllByProps({ 'aria-label': '确认购买未解锁种子' })).toHaveLength(0);
    expect(JSON.stringify(renderer.toJSON())).toContain('没有可购买的未解锁作物种子');
  });

  it('renders land details runtime gate without fake land rows', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="lands"
        initialLandDetails={{
          status: 'not_migrated',
          message: '土地详情需要迁移 /api/lands 与真实游戏运行时读取命令。',
          lands: [],
          actions: [
            { id: 'rush', label: '一键催熟', enabled: false, reason: 'not migrated' },
            { id: 'fertilize_normal', label: '一键无机肥', enabled: false, reason: 'not migrated' },
            { id: 'fertilize_organic', label: '一键有机肥', enabled: false, reason: 'not migrated' },
          ],
        }}
      />,
    );

    expect(html).toContain('土地详情');
    expect(html).toContain('not_migrated');
    expect(html).toContain('快捷操作');
    expect(html).not.toContain('一键催熟');
    expect(html).not.toContain('一键无机肥');
    expect(html).not.toContain('一键有机肥');
    expect(html).not.toContain('刷新土地');
    expect(html).not.toContain('升级土地');
    expect(html).toContain('没有土地数据');
    expect(html).toContain('aria-expanded="false"');
  });

  it('renders the land rush dialog controls copied from the reference flow', () => {
    const html = renderToStaticMarkup(
      <LandRushDialog
        open
        busy={false}
        disabled={false}
        lands={[
          { id: '1', landId: 1, plantName: '白萝卜', status: 'growing', statusLabel: '生长中', matureInSec: 300, canHarvest: false } as any,
        ]}
        onClose={() => undefined}
        onSubmit={() => undefined}
      />,
    );

    expect(html).toContain('role="dialog"');
    expect(html).toContain('一键催熟');
    expect(html).toContain('自动筛选');
    expect(html).toContain('手动选择');
    expect(html).toContain('成熟阈值(秒)');
    expect(html).toContain('2分钟');
    expect(html).toContain('5分钟');
    expect(html).toContain('10分钟');
    expect(html).toContain('催熟肥料');
    expect(html).toContain('无机');
    expect(html).toContain('有机');
    expect(html).toContain('催熟联动收获');
  });

  it('renders manual land rush choices for eligible lands', () => {
    const html = renderToStaticMarkup(
      <LandRushDialog
        open
        initialSelectionMode="manual"
        busy={false}
        disabled={false}
        lands={[
          { id: '1', landId: 1, plantName: '白萝卜', status: 'growing', statusLabel: '生长中', matureInSec: 300, canHarvest: false } as any,
          { id: '2', landId: 2, plantName: '胡萝卜', status: 'mature', statusLabel: '已成熟', matureInSec: 0, canHarvest: true } as any,
        ]}
        onClose={() => undefined}
        onSubmit={() => undefined}
      />,
    );

    expect(html).toContain('可催熟地块');
    expect(html).toContain('全选');
    expect(html).toContain('#1 白萝卜 · 5分钟后成熟');
    expect(html).not.toContain('#2 胡萝卜');
  });

  it('renders runtime land rows when backend returns real land data', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="lands"
        initialLandDetails={{
          status: 'runtime',
          message: '土地详情已从游戏运行时读取。',
          farmType: 'own',
          totalGrids: 1,
          lands: [
            { id: '1', landId: 1, landLevel: 1, plantName: '白萝卜', status: 'mature', statusLabel: '已成熟', canHarvest: true },
          ],
          actions: [{ id: 'rush', label: '一键催熟', enabled: true, reason: '' }],
        }}
      />,
    );

    expect(html).toContain('白萝卜');
    expect(html).toContain('已成熟');
    expect(html).not.toContain('没有土地数据');
  });

  it('polls land details through the HTTP polling transport', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');

    expect(source).toMatch(/import \{ [^}]*pollLand[^}]* \} from '\.\.\/lib\/pollApi'/);
    expect(source).toContain("import { createPollingController } from '../lib/pollingController'");
    expect(source).not.toContain('FarmLandDetails');
    expect(source).not.toContain('FarmLandDetailsSince');
  });

  it('restarts land polling with an empty revision for a forced full refresh', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');
    const refreshSource = source.match(/const refreshLandDetails[\s\S]*?\}, \[\]\);/)?.[0] || '';

    expect(refreshSource).toContain("landRevision.current = ''");
    expect(refreshSource).toContain('polling.stop();');
    expect(refreshSource).toContain('polling.start();');
    expect(refreshSource).not.toContain('polling.refresh();');
    expect(refreshSource.indexOf("landRevision.current = ''")).toBeLessThan(refreshSource.indexOf('polling.stop();'));
    expect(refreshSource.indexOf('polling.stop();')).toBeLessThan(refreshSource.indexOf('polling.start();'));
  });

  it('stops land polling after an unauthorized HTTP response', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');

    expect(source).toContain("import { PollApiError, pollLand } from '../lib/pollApi'");
    expect(source).toContain('shouldStop: (error) => error instanceof PollApiError && error.status === 401,');
  });

  it('renders compact enriched runtime land cards', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="lands"
        initialLandDetails={{
          status: 'runtime',
          message: '土地详情已从游戏运行时读取。',
          farmType: 'own',
          totalGrids: 1,
          lands: [
            {
              id: '1',
              landId: 1,
              landLevel: 1,
              landTypeLabel: '紫金土地',
              plantName: '白萝卜',
              displayPlantName: '黄金白萝卜',
              status: 'growing',
              statusLabel: '生长中',
              matureEtaText: '预计 04:31:45 后成熟',
              currentSeason: 2,
              totalSeason: 3,
              landSize: 1,
              occupancyPlantSize: 2,
              needWater: true,
              needWeed: true,
              needBug: true,
              needGoldenBug: true,
              canHarvest: false,
            } as any,
          ],
          actions: [{ id: 'rush', label: '一键催熟', enabled: true, reason: '' }],
        }}
      />,
    );

    expect(html).toContain('黄金白萝卜');
    expect(html).toContain('紫金土地');
    expect(html).toContain('预计 04:31:45 后成熟');
    expect(html).toContain('第 2/3 季');
    expect(html).toContain('2*2占地');
    expect(html).toContain('浇水');
    expect(html).toContain('除草');
    expect(html).toContain('杀虫');
    expect(html).toContain('金虫');
  });

  it('renders warehouse sell record controls and sell result toast copy', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="warehouse"
        initialWarehouse={{
          status: 'runtime',
          message: '仓库已从游戏运行时读取。',
          summary: {
            totalDistinct: 1,
            totalCount: 3,
            sellableDistinct: 1,
            sellableCount: 3,
            estimatedAllSellPrice: 720,
          },
          items: [
            {
              id: '41221:open:9001',
              itemId: 41221,
              name: '青梅',
              count: 3,
              category: 'fruit',
              categoryLabel: '果实',
              canSell: true,
              locked: false,
              estimatedSellPrice: 720,
            },
          ],
        }}
      />,
    );

    expect(html).toContain('出售记录');
    expect(html).toContain('出售选中');

    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');
    expect(source).toContain('warehouse-sell-toast');
    expect(source).toContain('FarmWarehouseSellRecords');
    expect(source).toContain('createPortal');
    expect(source).toContain('warehouse-records-backdrop');
    expect(source).toContain('className="icon-button warehouse-records-close"');
    expect(source).toContain('aria-label="关闭出售记录"');
    expect(css).toMatch(/\.warehouse-records-dialog\s*\{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\);/);
    expect(css).toMatch(/\.warehouse-records-close\s*\{[^}]*width:\s*34px;[^}]*height:\s*34px;[^}]*min-width:\s*34px;[^}]*min-height:\s*34px;/);
    expect(css).toMatch(/\.warehouse-records-close\s*\{[^}]*color:\s*#20271f;[^}]*background:\s*#d6ff67;[^}]*visibility:\s*visible;[^}]*opacity:\s*1;/);
    expect(source).toContain("mode: 'manual'");
    expect(source).toContain("mode: 'auto'");
    expect(source).toContain('出售数量');
    expect(source).toContain('出售次数');
    expect(source).toContain('出售金额');
  });

  it('renders runtime season labels for single-season crops too', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="lands"
        initialLandDetails={{
          status: 'runtime',
          message: '土地详情已从游戏运行时读取。',
          farmType: 'own',
          totalGrids: 1,
          lands: [
            {
              id: '2',
              landId: 2,
              plantName: '繁星花',
              status: 'growing',
              statusLabel: '生长中',
              matureEtaText: '预计 09:25:24 后成熟',
              currentSeason: 1,
              totalSeason: 1,
              canHarvest: false,
            } as any,
          ],
          actions: [{ id: 'rush', label: '一键催熟', enabled: true, reason: '' }],
        }}
      />,
    );

    expect(html).toContain('繁星花');
    expect(html).toContain('第 1/1 季');
  });

  it('renders countdown text from maturity seconds', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="lands"
        initialLandDetails={{
          status: 'runtime',
          message: '土地详情已从游戏运行时读取。',
          lands: [
            {
              id: '1',
              landId: 1,
              plantName: '白萝卜',
              status: 'growing',
              statusLabel: '生长中',
              matureInSec: 3661,
              canHarvest: false,
            } as any,
          ],
          actions: [],
        }}
      />,
    );

    expect(html).toContain('预计 01:01:01 后成熟');
  });

  it('renders mutation icon and type label on mutated land cards', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="lands"
        initialLandDetails={{
          status: 'runtime',
          message: '土地详情已从游戏运行时读取。',
          lands: [
            {
              id: '15',
              landId: 15,
              plantName: '白萝卜',
              displayPlantName: '黄金·白萝卜',
              status: 'growing',
              statusLabel: '生长中',
              hasMutation: true,
              mutationLabel: '黄金',
              mutationIconUrl: 'data:image/png;base64,abc',
              mutationImageUrl: 'data:image/png;base64,plant',
              canHarvest: false,
            } as any,
          ],
          actions: [],
        }}
      />,
    );

    expect(html).toContain('黄金·白萝卜');
    expect(html).toContain('变异·黄金');
    expect(html).toContain('alt="黄金"');
    expect(html).toContain('src="data:image/png;base64,abc"');
  });

  it('renders every mutation icon in runtime order on a land card', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="lands"
        initialLandDetails={{
          status: 'runtime',
          message: '土地详情已从游戏运行时读取。',
          lands: [
            {
              id: '16',
              landId: 16,
              plantName: '荷花',
              status: 'growing',
              statusLabel: '生长中',
              hasMutation: true,
              mutationTypes: [
                { typeId: 1, name: '黄金', iconUrl: 'data:image/png;base64,gold' },
                { typeId: 2, name: '神秘', iconUrl: 'data:image/png;base64,mystery' },
              ],
            } as any,
          ],
          actions: [],
        }}
      />,
    );

    const gold = html.indexOf('src="data:image/png;base64,gold"');
    const mystery = html.indexOf('src="data:image/png;base64,mystery"');
    expect(gold).toBeGreaterThan(-1);
    expect(mystery).toBeGreaterThan(gold);
  });

  it('renders a default visual icon without duplicating the land id', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="lands"
        initialLandDetails={{
          status: 'runtime',
          message: '土地详情已从游戏运行时读取。',
          farmType: 'own',
          totalGrids: 1,
          lands: [
            {
              id: '1',
              landId: 1,
              landTypeLabel: '红土地',
              status: 'empty',
              statusLabel: '空地',
              matureEtaText: '空地',
              canHarvest: false,
            } as any,
          ],
          actions: [{ id: 'rush', label: '一键催熟', enabled: true, reason: '' }],
        }}
      />,
    );

    expect(html).toContain('land-visual land-visual-empty');
    expect(html).toContain('aria-label="默认土地图标"');
    expect((html.match(/#1/g) || []).length).toBe(1);
  });

  it('renders each stage image from its trimmed alpha bounds', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');
    const cropRect = (AssetsLandModule as Record<string, unknown>).landArtworkCropRect;

    expect(source).toContain('function LandStageImage');
    expect(source).toContain('drawTrimmedArtwork');
    expect(source).toContain('<canvas');
    expect(source).toContain('src={src}');
    expect(source).not.toContain('canvas.toDataURL');
    expect(cropRect).toBeTypeOf('function');
    if (typeof cropRect === 'function') {
      expect(cropRect({ width: 150, height: 150, minX: 47, maxX: 101, minY: 71, maxY: 112 }))
        .toEqual({ x: 35, y: 59, width: 79, height: 66 });
    }
    expect(css).toContain('.app-shell-remote .land-visual-image');
    expect(css).toContain('width: 56px;');
    expect(css).toContain('height: 56px;');
    expect(css).not.toContain('transform: translateY(var(--land-art-offset, 0%))');
  });

  it('ignores faint alpha pixels when measuring the visible artwork bounds', () => {
    const visibleBounds = (AssetsLandModule as Record<string, unknown>).landArtworkVisibleBounds;

    expect(visibleBounds).toBeTypeOf('function');
    if (typeof visibleBounds !== 'function') return;

    const pixels = new Uint8ClampedArray(10 * 10 * 4);
    const setAlpha = (x: number, y: number, alpha: number) => {
      pixels[(y * 10 + x) * 4 + 3] = alpha;
    };
    setAlpha(0, 0, 13);
    setAlpha(9, 9, 127);
    setAlpha(2, 3, 128);
    setAlpha(6, 7, 255);

    expect(visibleBounds(10, 10, pixels)).toMatchObject({
      width: 10,
      height: 10,
      minX: 2,
      maxX: 6,
      minY: 3,
      maxY: 7,
    });
  });

  it('centers visible alpha bounds for asymmetric artwork without clipping them', () => {
    const drawPlacement = (AssetsLandModule as Record<string, unknown>).landArtworkDrawPlacement;

    expect(drawPlacement).toBeTypeOf('function');
    if (typeof drawPlacement !== 'function') return;

    const bounds = {
      width: 200,
      height: 200,
      minX: 20,
      maxX: 180,
      minY: 20,
      maxY: 180,
      alphaCenterX: 50,
      alphaCenterY: 140,
    };
    const crop = { x: 4, y: 4, width: 192, height: 192 };
    const placement = drawPlacement(bounds, crop, 148);

    expect(placement).toBeTruthy();
    if (!placement) return;
    const scale = placement.width / crop.width;
    const visibleCenterX = placement.x + (((bounds.minX + bounds.maxX) / 2 - crop.x) * scale);
    const visibleCenterY = placement.y + (((bounds.minY + bounds.maxY) / 2 - crop.y) * scale);
    expect(visibleCenterX).toBeCloseTo(74, 5);
    expect(visibleCenterY).toBeCloseTo(74, 5);
    expect(placement.x + (bounds.minX - crop.x) * scale).toBeGreaterThanOrEqual(4);
    expect(placement.x + (bounds.maxX - crop.x) * scale).toBeLessThanOrEqual(144);
    expect(placement.y + (bounds.minY - crop.y) * scale).toBeGreaterThanOrEqual(4);
    expect(placement.y + (bounds.maxY - crop.y) * scale).toBeLessThanOrEqual(144);
  });

  it('keeps remote land results inside a vertical scroll region', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(source).toContain('land-details-content');
    expect(source).toContain('land-details-actions');
    expect(source).toContain('land-details-results');
    expect(css).toContain('.app-shell-remote .land-details-section');
    expect(css).toContain('.app-shell-remote .land-details-results');
    expect(css).toContain('overflow-y: auto;');
    expect(css).toContain('overscroll-behavior: contain;');
  });

  it('sorts land cards by land id and renders figure-style card actions', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="lands"
        initialLandDetails={{
          status: 'runtime',
          message: '土地详情已从游戏运行时读取。',
          lands: [
            { id: '3', landId: 3, status: 'empty', statusLabel: '空地', canHarvest: false },
            { id: '1', landId: 1, status: 'empty', statusLabel: '空地', canHarvest: false },
            { id: '2', landId: 2, status: 'empty', statusLabel: '空地', canHarvest: false },
          ],
          actions: [],
        }}
      />,
    );

    expect(html.indexOf('#1')).toBeLessThan(html.indexOf('#2'));
    expect(html.indexOf('#2')).toBeLessThan(html.indexOf('#3'));
    expect(html).toContain('land-card-actions');
    expect(html).toContain('>施肥</span>');
    expect(html).toContain('>铲除</span>');
    expect(html).not.toContain('>无机</span>');
    expect(html).not.toContain('>有机</span>');
  });

  it('renders compact mobile land countdowns and labelled icon actions', () => {
    const compactCountdown = (AssetsLandModule as Record<string, unknown>).landCompactCountdownText;

    expect(compactCountdown).toBeTypeOf('function');
    if (typeof compactCountdown !== 'function') return;
    expect(compactCountdown({ status: 'mature', canHarvest: true }, 0)).toBe('可收');
    expect(compactCountdown({ status: 'growing', matureInSec: 205 }, 0)).toBe('03:25');

    const html = renderToStaticMarkup(
      <LandDetailsPanel
        loading={false}
        error=""
        nowMs={0}
        onRefresh={() => undefined}
        payload={{
          status: 'runtime',
          lands: [{ id: '1', landId: 1, plantName: '鹭草', status: 'growing', statusLabel: '生长中', matureInSec: 205, canHarvest: false }],
          actions: [],
        }}
      />,
    );

    expect(html).toContain('land-countdown-compact');
    expect(html).toContain('aria-label="选择肥料类型"');
    expect(html).toContain('aria-label="铲除作物"');
    expect(html).not.toContain('aria-label="施用无机肥"');
    expect(html).not.toContain('aria-label="施用有机肥"');
  });

  it('uses two readable columns for remote mobile land cards', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toMatch(/\.land-grid\s*\{[\s\S]*grid-template-columns:\s*repeat\(4,\s*minmax\(0,\s*1fr\)\);/);
    expect(css).toMatch(/\.app-shell-remote \.land-grid\s*\{[\s\S]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/);
    expect(css).toMatch(/\.app-shell-remote \.land-tile\s*\{[\s\S]*min-height:\s*270px;/);
    expect(css).toMatch(/\.app-shell-remote \.land-visual\s*\{[\s\S]*height:\s*82px;/);
    expect(css).toMatch(/\.land-card-actions\s*\{[\s\S]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/);
    expect(css).toMatch(/\.land-card-action-fertilize\s*\{[\s\S]*display:\s*inline-flex;/);
    expect(css).not.toContain('land-card-action-mode');
  });

  it('uses the shared fertilizer trigger without rendering a land progress bar', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');
    const html = renderToStaticMarkup(
      <LandDetailsPanel
        loading={false}
        error=""
        nowMs={0}
        onRefresh={() => undefined}
        payload={{
          status: 'runtime',
          lands: [{
            id: '1', landId: 1, plantName: '白萝卜', status: 'growing', statusLabel: '生长中',
            matureInSec: 205, currentSeason: 2, totalSeason: 3,
            landTypeLabel: '紫金土地', canHarvest: false,
          }],
          actions: [],
        }}
      />,
    );

    expect(html).toContain('aria-label="选择肥料类型"');
    expect(html).not.toContain('land-growth-track');
    expect(source).not.toContain('landGrowthPercent');
    expect(css).not.toContain('.land-growth-track');
  });

  it('centers the desktop fertilizer chooser while retaining the remote bottom-sheet override', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toMatch(/\.land-fertilizer-drawer-backdrop\s*\{[\s\S]*display:\s*grid;[\s\S]*place-items:\s*center;/);
    expect(css).toMatch(/\.land-fertilizer-drawer\s*\{[\s\S]*width:\s*min\(440px,\s*calc\(100vw - 32px\)\);/);
    expect(css).toMatch(/\.app-shell-remote \.land-rush-drawer-backdrop,\s*\.app-shell-remote \.land-fertilizer-drawer-backdrop,[\s\S]*\{[\s\S]*place-items:\s*end stretch;/);
  });

  it('opens the fertilizer chooser and dispatches the selected fertilizer mode for its land', () => {
    const onLandCardAction = vi.fn();
    const renderer = create(
      <LandDetailsPanel
        loading={false}
        error=""
        nowMs={0}
        onRefresh={() => undefined}
        onLandCardAction={onLandCardAction}
        payload={{
          status: 'runtime',
          lands: [{ id: '1', landId: 1, plantName: '白萝卜', status: 'growing', statusLabel: '生长中', canHarvest: false }],
          actions: [],
        }}
      />,
    );

    act(() => {
      renderer.root.findByProps({ 'aria-label': '选择肥料类型' }).props.onClick();
    });
    expect(renderer.root.findByProps({ role: 'dialog' }).props['aria-label']).toBe('对 #1 白萝卜施肥');

    act(() => {
      renderer.root.findAllByType('button').find((node) => node.children.join('') === '取消')?.props.onClick();
    });
    expect(renderer.root.findAllByProps({ role: 'dialog' })).toHaveLength(0);

    act(() => {
      renderer.root.findByProps({ 'aria-label': '选择肥料类型' }).props.onClick();
    });
    act(() => {
      renderer.root.findByProps({ 'aria-label': '对 #1 白萝卜施用有机肥' }).props.onClick();
    });
    expect(onLandCardAction).toHaveBeenCalledWith(
      'fertilize',
      expect.objectContaining({ landId: 1 }),
      'organic',
    );
    expect(renderer.root.findAllByProps({ role: 'dialog' })).toHaveLength(0);
  });

  it('does not open the fertilizer chooser when land card actions are disabled', () => {
    const renderer = create(
      <LandDetailsPanel
        loading={false}
        error=""
        nowMs={0}
        cardActionsDisabled
        onRefresh={() => undefined}
        payload={{
          status: 'runtime',
          lands: [{ id: '1', landId: 1, plantName: '白萝卜', status: 'growing', statusLabel: '生长中', canHarvest: false }],
          actions: [],
        }}
      />,
    );

    const trigger = renderer.root.findByProps({ 'aria-label': '选择肥料类型' });
    expect(trigger.props.disabled).toBe(true);
    act(() => {
      trigger.props.onClick();
    });
    expect(renderer.root.findAllByProps({ role: 'dialog' })).toHaveLength(0);
  });

  it('places the asset switcher above a full-width content panel', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toMatch(/\.assets-land-layout\s*\{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\);/);
    expect(css).toMatch(/\.asset-tabs\s*\{[^}]*grid-template-columns:\s*repeat\(4,\s*minmax\(0,\s*1fr\)\);/);
  });

  it('constrains the atlas purchase confirmation dialog on small screens', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toContain('.atlas-purchase-dialog');
    expect(css).toContain('width: min(460px, calc(100vw - 32px))');
  });

  it('defines compact remote asset grids and bounded sheets', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toContain('.app-shell-remote .warehouse-mobile-list');
    expect(css).toContain('.app-shell-remote .warehouse-desktop-table');
    expect(css).toContain('.app-shell-remote .land-grid');
    expect(css).toContain('.app-shell-remote .atlas-card-grid');
    expect(css).toContain('.app-shell-remote .land-rush-drawer');
    expect(css).toContain('.app-shell-remote .warehouse-records-backdrop > .warehouse-records-dialog');
    expect(css).toContain('.app-shell-remote .atlas-purchase-dialog');
    expect(css).toContain('max-height: 82dvh');
  });

  it('renders a labelled close control and scroll body for atlas purchases', () => {
    const html = renderToStaticMarkup(
      <AtlasPurchaseConfirmDialog
        open
        purchases={[{ seedId: 20002, seedName: '白萝卜种子', goodsId: 90002, price: 1, count: 3, requiredLevel: 1 }]}
        busy={false}
        error=""
        onCancel={() => undefined}
        onConfirm={() => undefined}
      />,
    );

    expect(html).toContain('aria-label="关闭确认购买"');
    expect(html).toContain('atlas-purchase-body');
    expect(html).toContain('确认购买');
  });

  it('renders stable local resource URLs for land and mutation images', () => {
    const html = renderToStaticMarkup(
      <LandDetailsPanel
        loading={false}
        error=""
        nowMs={Date.now()}
        onRefresh={() => undefined}
        payload={{
          status: 'runtime',
          lands: [{
            id: '1',
            landId: 1,
            plantName: '白萝卜',
            status: 'growing',
            statusLabel: '生长中',
            imageUrl: '/farm-assets/crop-stage',
            mutationIconUrl: '/farm-assets/mutation-icon',
            canHarvest: false,
          }],
          actions: [],
        }}
      />,
    );

    expect(html).toContain('src="/farm-assets/crop-stage"');
    expect(html).toContain('src="/farm-assets/mutation-icon"');
    expect(html).not.toContain('data:image/');
  });

  it('wires land card actions to runtime fertilize and shovel commands', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');

    expect(source).toContain('FarmFertilizeLand');
    expect(source).toContain('FarmShovelLands');
    expect(source).toContain("onLandCardAction?.('fertilize'");
    expect(source).toContain("onLandCardAction?.('shovel'");
  });

  it('renders a closed quick-action entry for all growing lands', () => {
    const html = renderToStaticMarkup(
      <LandDetailsPanel
        loading={false}
        error=""
        nowMs={Date.now()}
        onRefresh={() => undefined}
        onRush={() => undefined}
        onBulkFertilize={() => undefined}
        payload={{
          status: 'runtime',
          message: '土地详情已从游戏运行时读取。',
          lands: [
            { id: '1', landId: 1, plantName: '菠菜', status: 'growing', statusLabel: '生长中', matureInSec: 1200, canHarvest: false },
            { id: '2', landId: 2, plantName: '白萝卜', status: 'mature', statusLabel: '已成熟', matureInSec: 0, canHarvest: true },
          ],
          actions: [
            { id: 'rush', label: '一键催熟', enabled: true, reason: '' },
            { id: 'fertilize_normal', label: '一键无机肥', enabled: true, reason: '' },
            { id: 'fertilize_organic', label: '一键有机肥', enabled: true, reason: '' },
          ],
        }}
      />,
    );

    expect(html).toContain('快捷操作');
    expect(html).not.toContain('一键催熟');
    expect(html).not.toContain('一键无机肥');
    expect(html).not.toContain('一键有机肥');
  });

  it('wires bulk fertilizer actions through FarmLandRush for growing lands', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');

    expect(source).toContain('function submitBulkFertilizer');
    expect(source).toContain('isGrowingLandForFertilizer');
    expect(source).toContain("onBulkFertilize?.('normal')");
    expect(source).toContain("onBulkFertilize?.('organic')");
    expect(source).toContain("fertilizerSubmissionScope: 'manual'");
    expect(source).toContain('harvestLinkEnabled: false');
  });

  it('deduplicates four-grid fertilizer targets by occupancy anchor', () => {
    const fourGridLand = (landId: number) => ({
      id: String(landId),
      landId,
      occupancyAnchorLandId: 5,
      plantName: '哈哈南瓜',
      status: 'growing',
      statusLabel: '生长中',
      canHarvest: false,
    });

    expect(fertilizerTargetLandIds([
      fourGridLand(1),
      fourGridLand(2),
      fourGridLand(5),
      fourGridLand(6),
      { id: '9', landId: 9, plantName: '白萝卜', status: 'growing', statusLabel: '生长中', canHarvest: false },
    ])).toEqual([5, 9]);
  });

  it('selects planted land anchors for one-click shovel', () => {
    expect(bulkShovelTargetLandIds([
      { id: '1', landId: 1, occupancyAnchorLandId: 1, plantName: '四格作物', status: 'growing', statusLabel: '生长中', canHarvest: false },
      { id: '2', landId: 2, occupancyAnchorLandId: 1, plantName: '四格作物', status: 'growing', statusLabel: '生长中', canHarvest: false },
      { id: '3', landId: 3, plantName: '白萝卜', status: 'mature', statusLabel: '已成熟', canHarvest: true },
      { id: '4', landId: 4, status: 'empty', statusLabel: '空地', canHarvest: false },
      { id: '5', landId: 5, seedId: 20001, status: 'locked', statusLabel: '锁定', canHarvest: false },
    ] as any)).toEqual([1, 3]);
  });

  it('opens the seven quick actions and confirms bulk shovel before submitting', () => {
    const onRunAutomationTask = vi.fn();
    const onBulkShovel = vi.fn();
    const renderer = create(
      <LandDetailsPanel
        loading={false}
        error=""
        nowMs={0}
        onRefresh={() => undefined}
        onRush={() => undefined}
        onBulkFertilize={() => undefined}
        onRunAutomationTask={onRunAutomationTask}
        onBulkShovel={onBulkShovel}
        payload={{
          status: 'runtime',
          farmType: 'own',
          lands: [
            { id: '1', landId: 1, plantName: '菠菜', status: 'growing', statusLabel: '生长中', canHarvest: false },
            { id: '2', landId: 2, plantName: '白萝卜', status: 'mature', statusLabel: '已成熟', canHarvest: true },
          ],
          actions: [
            { id: 'rush', label: '一键催熟', enabled: true, reason: '' },
            { id: 'fertilize_normal', label: '一键无机肥', enabled: true, reason: '' },
            { id: 'fertilize_organic', label: '一键有机肥', enabled: true, reason: '' },
          ],
        }}
      />,
    );

    act(() => renderer.root.findByProps({ 'aria-label': '打开快捷操作' }).props.onClick());

    const menu = renderer.root.findByProps({ id: 'land-quick-actions-menu' });
    expect(menu.props.role).toBe('menu');
    for (const label of ['一键务农', '一键收获', '一键种植', '一键铲除', '一键催熟', '一键无机肥', '一键有机肥']) {
      expect(renderer.root.findByProps({ 'aria-label': label }).props.role).toBe('menuitem');
    }

    act(() => renderer.root.findByProps({ 'aria-label': '一键铲除' }).props.onClick());

    expect(onBulkShovel).not.toHaveBeenCalled();
    expect(renderer.root.findByProps({ role: 'alertdialog' }).findByType('p').children.join('')).toContain('2 块有作物地块');

    act(() => renderer.root.findByProps({ 'aria-label': '确认铲除 2 块地' }).props.onClick());

    expect(onBulkShovel).toHaveBeenCalledWith([1, 2]);
    expect(onRunAutomationTask).not.toHaveBeenCalled();
  });

  it('runs each one-click farming command through the existing runtime task binding', async () => {
    landActions.runAutomationTask.mockResolvedValue({ ok: true, message: '一键务农已执行' });
    const renderer = create(
      <AssetsLandView
        initialTab="lands"
        initialCropAnalytics={{ items: [] }}
        initialAtlasPreview={{ sections: [] }}
        initialLandDetails={{
          status: 'runtime',
          farmType: 'own',
          lands: [{ id: '1', landId: 1, plantName: '菠菜', status: 'growing', statusLabel: '生长中', canHarvest: false }],
          actions: [
            { id: 'rush', label: '一键催熟', enabled: true, reason: '' },
            { id: 'fertilize_normal', label: '一键无机肥', enabled: true, reason: '' },
            { id: 'fertilize_organic', label: '一键有机肥', enabled: true, reason: '' },
          ],
        }}
      />,
    );

    for (const [label, taskId] of [
      ['一键务农', 'own_base'],
      ['一键收获', 'own_collect'],
      ['一键种植', 'own_plant'],
    ]) {
      act(() => renderer.root.findByProps({ 'aria-label': '打开快捷操作' }).props.onClick());
      await act(async () => {
        renderer.root.findByProps({ 'aria-label': label }).props.onClick();
        await Promise.resolve();
      });
      expect(landActions.runAutomationTask).toHaveBeenCalledWith(taskId);
    }

    expect(renderer.root.findByProps({ role: 'status' }).children.join('')).toContain('一键务农已执行');
  });

  it('submits confirmed one-click shovel targets through the existing runtime binding', async () => {
    landActions.shovel.mockResolvedValue({ ok: true, message: '铲除已提交' });
    const renderer = create(
      <AssetsLandView
        initialTab="lands"
        initialCropAnalytics={{ items: [] }}
        initialAtlasPreview={{ sections: [] }}
        initialLandDetails={{
          status: 'runtime',
          farmType: 'own',
          lands: [
            { id: '1', landId: 1, plantName: '菠菜', status: 'growing', statusLabel: '生长中', canHarvest: false },
            { id: '2', landId: 2, plantName: '白萝卜', status: 'mature', statusLabel: '已成熟', canHarvest: true },
          ],
          actions: [
            { id: 'rush', label: '一键催熟', enabled: true, reason: '' },
            { id: 'fertilize_normal', label: '一键无机肥', enabled: true, reason: '' },
            { id: 'fertilize_organic', label: '一键有机肥', enabled: true, reason: '' },
          ],
        }}
      />,
    );

    act(() => renderer.root.findByProps({ 'aria-label': '打开快捷操作' }).props.onClick());
    act(() => renderer.root.findByProps({ 'aria-label': '一键铲除' }).props.onClick());
    await act(async () => {
      renderer.root.findByProps({ 'aria-label': '确认铲除 2 块地' }).props.onClick();
      await Promise.resolve();
    });

    expect(landActions.shovel).toHaveBeenCalledWith({ landIds: [1, 2] });
    expect(renderer.root.findByProps({ role: 'status' }).children.join('')).toContain('铲除已提交');
  });

  it('disables competing land controls while a quick command is in flight', async () => {
    let resolveTask: (result: { ok: boolean; message: string }) => void = () => undefined;
    landActions.runAutomationTask.mockImplementation(() => new Promise((resolve) => {
      resolveTask = resolve;
    }));
    const renderer = create(
      <AssetsLandView
        initialTab="lands"
        initialCropAnalytics={{ items: [] }}
        initialAtlasPreview={{ sections: [] }}
        initialLandDetails={{
          status: 'runtime',
          farmType: 'own',
          lands: [{ id: '1', landId: 1, plantName: '菠菜', status: 'growing', statusLabel: '生长中', canHarvest: false }],
          actions: [
            { id: 'rush', label: '一键催熟', enabled: true, reason: '' },
            { id: 'fertilize_normal', label: '一键无机肥', enabled: true, reason: '' },
            { id: 'fertilize_organic', label: '一键有机肥', enabled: true, reason: '' },
          ],
        }}
      />,
    );

    act(() => renderer.root.findByProps({ 'aria-label': '打开快捷操作' }).props.onClick());
    act(() => renderer.root.findByProps({ 'aria-label': '一键务农' }).props.onClick());
    act(() => renderer.root.findByProps({ 'aria-label': '打开快捷操作' }).props.onClick());

    const rushItem = renderer.root.findByProps({ 'aria-label': '一键催熟' });
    expect(rushItem.props.disabled).toBe(true);
    expect(rushItem.findByType('small').children.join('')).toContain('正在执行其他土地操作');
    expect(renderer.root.findByProps({ 'aria-label': '铲除作物' }).props.disabled).toBe(true);

    await act(async () => {
      resolveTask({ ok: true, message: '一键务农已完成' });
      await Promise.resolve();
    });
  });

  it('styles land quick actions as a desktop menu and remote mobile sheet', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toContain('.land-quick-actions');
    expect(css).toContain('.land-quick-menu');
    expect(css).toContain('.land-quick-menu-backdrop');
    expect(css).toMatch(/\.app-shell-remote \.land-quick-menu\s*\{[\s\S]*max-height:\s*82dvh;/);
    expect(css).toMatch(/\.app-shell-remote \.land-quick-menu-item\s*\{[\s\S]*min-height:\s*42px;/);
    expect(css).toMatch(/\.app-shell-remote \.land-quick-confirm\s*\{[\s\S]*border-radius:\s*14px 14px 0 0;/);
  });

  it('keeps existing land cards visible while a background refresh is loading', () => {
    const html = renderToStaticMarkup(
      <LandDetailsPanel
        loading
        error=""
        nowMs={Date.now()}
        onRefresh={() => undefined}
        payload={{
          status: 'runtime',
          message: '土地详情已从游戏运行时读取。',
          lands: [
            { id: '1', landId: 1, plantName: '白萝卜', status: 'growing', statusLabel: '生长中', matureInSec: 30, canHarvest: false },
          ],
          actions: [],
        }}
      />,
    );

    expect(html).toContain('白萝卜');
    expect(html).toContain('land-grid');
    expect(html).not.toContain('正在读取土地状态');
  });

  it('does not pass transient land loading state to the land rush submit lock when lands already exist', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');

    expect(source).toContain("disabled={landDetails?.farmType === 'friend'}");
    expect(source).not.toContain("disabled={landLoading || landDetails?.farmType === 'friend'}");
  });

  it('renders warehouse runtime gate without fake item rows', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="warehouse"
        initialWarehouse={{
          status: 'not_migrated',
          message: '仓库需要迁移 /api/warehouse、刷新和出售运行时命令。',
          items: [],
          actions: [
            { id: 'refresh', label: '刷新仓库', enabled: false, reason: 'not migrated' },
            { id: 'sell', label: '出售选中', enabled: false, reason: 'not migrated' },
          ],
        }}
      />,
    );

    expect(html).toContain('仓库');
    expect(html).toContain('not_migrated');
    expect(html).toContain('刷新仓库');
    expect(html).toContain('出售选中');
    expect(html).toContain('没有仓库物品数据');
    expect(html).toContain('disabled=""');
  });

  it('renders runtime warehouse items when backend returns a snapshot', () => {
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="warehouse"
        initialWarehouse={{
          status: 'runtime',
          message: '仓库已从游戏运行时读取。',
          summary: {
            totalDistinct: 1,
            totalCount: 5,
            sellableDistinct: 1,
            sellableCount: 5,
            estimatedAllSellPrice: 10,
            categoryList: [{ key: 'fruit', label: '果实', distinct: 1, count: 5 }],
          },
          items: [
            { id: '40002:0', itemId: 40002, name: '白萝卜', count: 5, category: 'fruit', categoryLabel: '果实', canSell: true, locked: false, estimatedSellPrice: 10, imageUrl: 'data:image/png;base64,abc' },
          ],
          actions: [
            { id: 'refresh', label: '刷新仓库', enabled: true, reason: '' },
            { id: 'sell', label: '出售选中', enabled: true, reason: '' },
          ],
        }}
      />,
    );

    expect(html).toContain('白萝卜');
    expect(html).toContain('果实');
    expect(html).toContain('预计 10');
    expect(html).toContain('仓库分类');
    expect(html).toContain('warehouse-desktop-table');
    expect(html).toContain('warehouse-mobile-list');
    expect(html).toContain('warehouse-mobile-row');
    expect(html).not.toContain('warehouse-item-image');
    expect(html).not.toContain('src="data:image/png;base64,abc"');
    expect(html).not.toContain('40002');
    expect(html).toContain('type="checkbox"');
    expect(html).toContain('自动出售设置');
    expect(html).toContain('出售选中');
    expect(html).not.toContain('自动出售当前分类');
    expect(html).not.toContain('没有仓库物品数据');
  });

  it('keeps the remote warehouse list in a bounded vertical scroll region', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toMatch(/\.app-shell-remote \.warehouse-section\s*\{[^}]*height:\s*100%;[^}]*overflow:\s*hidden;/);
    expect(css).toMatch(/\.app-shell-remote \.asset-table-wrap:has\(\.warehouse-mobile-list\)\s*\{[^}]*min-height:\s*0;[^}]*overflow-x:\s*hidden;[^}]*overflow-y:\s*auto;[^}]*overscroll-behavior:\s*contain;/);
  });

  it('keeps mobile records and settings surfaces in named body regions', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');

    expect(source).toContain('warehouse-records-mobile-list');
    expect(source).toContain('warehouse-records-body');
    expect(source).toContain('warehouse-settings-body');
    expect(source).toContain('land-rush-body');
  });

  it('hides system resources from the warehouse while retaining ordinary items', () => {
    const hiddenNames = [
      '普通化肥容器', '有机化肥容器', '种植经验', '普通收藏点',
      '典藏收藏点', '金币', '点券', '金豆', '金豆豆',
    ];
    const html = renderToStaticMarkup(
      <AssetsLandView
        initialTab="warehouse"
        initialWarehouse={{
          status: 'runtime',
          message: '仓库已从游戏运行时读取。',
          items: [
            ...hiddenNames.map((name, index) => ({
              id: `system:${index}`,
              itemId: index + 1,
              name,
              count: 1,
              category: 'tool',
              categoryLabel: '道具',
              canSell: false,
              locked: false,
              estimatedSellPrice: 0,
            })),
            {
              id: '40002:0',
              itemId: 40002,
              name: '白萝卜',
              count: 5,
              category: 'fruit',
              categoryLabel: '果实',
              canSell: true,
              locked: false,
              estimatedSellPrice: 10,
            },
          ],
        }}
      />,
    );

    for (const name of hiddenNames) expect(html).not.toContain(name);
    expect(html).toContain('白萝卜');
  });

  it('keeps warehouse auto sell settings in source with fixed category options', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');

    expect(source).toContain('自动出售设置');
    expect(source).toContain('出售间隔');
    expect(source).toContain('分钟');
    expect(source).toContain('intervalMinute: 60');
    expect(source).not.toContain('intervalHour');
    expect(source).not.toContain('window.setInterval(() => onAutoSell');
    expect(source).toContain('果实');
    expect(source).toContain('超变果实');
    expect(source).toContain('种子');
    expect(source).toContain('道具');
    expect(source).toContain('WarehouseAutoSellSettings');
    expect(source).toContain('SaveWarehouseAutoSellSettings');
    expect(source).not.toContain('RuntimeSettings');
    expect(source).not.toContain('SaveRuntimeSettings');
    expect(source).not.toContain('submitAutoSellFiltered');
  });

  it('uses a compact grouped auto-sell form on remote phones', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(source).toContain('warehouse-settings-group warehouse-settings-execution');
    expect(source).toContain('warehouse-settings-toggle');
    expect(source).toContain('warehouse-settings-input-unit');
    expect(css).toContain('.app-shell-remote .warehouse-settings-group');
    expect(css).toContain('.app-shell-remote .warehouse-settings-toggle');
    expect(css).toContain('.app-shell-remote .warehouse-settings-input-unit');
    expect(css).toContain('.app-shell-remote .warehouse-settings-options');
    expect(css).toMatch(/\.app-shell-remote \.warehouse-settings-group \.warehouse-settings-field\s*\{[^}]*display:\s*flex;[^}]*justify-content:\s*space-between;/);
  });

  it('sizes the remote auto-sell drawer to its content', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toContain(':not(.social-dialog):not(.warehouse-settings-dialog),');
    expect(css).toMatch(/\.app-shell-remote \.dialog-backdrop > \.warehouse-settings-dialog\s*\{[^}]*height:\s*auto;/);
  });

  it('does not auto refresh warehouse when assets view opens', () => {
    const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');

    expect(source).not.toContain('useEffect(() => {\n    if (initialWarehouse) return;');
    expect(source).toContain('const [warehouseLoading, setWarehouseLoading] = useState(false)');
  });
});
