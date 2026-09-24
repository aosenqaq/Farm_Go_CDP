import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { AppShell } from './AppShell';

describe('AppShell navigation', () => {
  it('uses grouped farm feature navigation', () => {
    const html = renderToStaticMarkup(
      <AppShell
        activeTab="workspace"
        account={{ nickname: 'Dpo.L', gid: 123456, avatarUrl: 'https://example.test/avatar.png', confirmed: true }}
        onTabChange={() => undefined}
      >
        <div>content</div>
      </AppShell>,
    );

    expect(html).toContain('工作台');
    expect(html).toContain('农场自动化');
    expect(html).toContain('资产与土地');
    expect(html).toContain('好友社交');
    expect(html).toContain('账户状态');
    expect(html).toContain('守护服务');
    expect(html).toContain('日志中心');
    expect(html).toContain('消息推送');
    expect(html).toContain('系统设置');
    expect(html).toContain('Farm Go');
    expect(html).not.toContain('Farm_Go');
    expect(html).not.toContain('本机诊断站');
    expect(html).not.toContain('账户与系统');
    expect(html).not.toContain('连接');
    expect(html).not.toContain('<span>诊断</span>');
    expect(html).not.toContain('<span>自动农场</span><span>调度中心</span>');
    expect(html).not.toContain('<span>仓库</span><span>好友</span>');
    expect(html).toContain('Dpo.L');
    expect(html).not.toContain('GId');
    expect(html).not.toContain('隐藏 GId');
    expect(html).not.toContain('显示 GId');
    expect(html).not.toContain('时长卡');
    expect(html).not.toContain('2027-07-13');
    expect(html).not.toContain('本次运行');
  });


  it('omits desktop-only management destinations in a remote console session', () => {
    const html = renderToStaticMarkup(
      <AppShell
        activeTab="workspace"
        remote
        update={{ available: true, latest: { name: 'v9.9.9' }, current: { name: 'v1.0.0' } } as any}
        onTabChange={() => undefined}
      >
        <div>content</div>
      </AppShell>,
    );

    expect(html).toContain('工作台');
    expect(html).toContain('账户状态');
    expect(html).not.toContain('守护服务');
    expect(html).not.toContain('日志中心');
    expect(html).not.toContain('消息推送');
    expect(html).not.toContain('系统设置');
    expect(html).not.toContain('发现新版本');
    expect(html).toContain('class="app-shell app-shell-remote"');
  });
});
