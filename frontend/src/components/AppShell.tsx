import { Bell, Bot, CircleUserRound, LayoutDashboard, ScrollText, Settings, ShieldCheck, UserRound, Users, Warehouse } from 'lucide-react';
import type { ReactNode } from 'react';
import { FallbackImage } from './FallbackImage';
import { UpdateAvailableToast } from './UpdateAvailableToast';
import type { UpdateStateDto } from '../lib/update';

export type Tab = 'workspace' | 'automation' | 'assets' | 'social' | 'account' | 'guard' | 'logs' | 'message_push' | 'settings';

export type ShellAccount = {
  gid?: number;
  nickname?: string;
  avatarUrl?: string;
  confirmed?: boolean;
};

type AppShellProps = {
  activeTab: Tab;
  remote?: boolean;
  account?: ShellAccount | null;
  update?: UpdateStateDto | null;
  onOpenUpdateDetails?: () => void;
  onTabChange: (tab: Tab) => void;
  children: ReactNode;
};

const nav = [
  { id: 'workspace' as const, label: '工作台', icon: LayoutDashboard },
  { id: 'automation' as const, label: '农场自动化', icon: Bot },
  { id: 'assets' as const, label: '资产与土地', icon: Warehouse },
  { id: 'social' as const, label: '好友社交', icon: Users },
  { id: 'account' as const, label: '账户状态', icon: CircleUserRound },
  { id: 'guard' as const, label: '守护服务', icon: ShieldCheck },
  { id: 'logs' as const, label: '日志中心', icon: ScrollText },
  { id: 'message_push' as const, label: '消息推送', icon: Bell },
  { id: 'settings' as const, label: '系统设置', icon: Settings },
];

export function AppShell({ activeTab, remote = false, account, update = null, onOpenUpdateDetails = () => undefined, onTabChange, children }: AppShellProps) {
  const visibleNav = remote ? nav.filter((item) => !['guard', 'logs', 'message_push', 'settings'].includes(item.id)) : nav;
  return (
    <div className={remote ? 'app-shell app-shell-remote' : 'app-shell'}>
      <aside className="sidebar">
        <div className="brand-block">
          <div className="brand-name">Farm Go</div>
        </div>
        <nav className="sidebar-nav" aria-label="主导航">
          {visibleNav.map((item) => {
            const Icon = item.icon;
            const active = item.id === activeTab;
            return (
              <button
                key={item.id}
                type="button"
                className={active ? 'nav-button nav-button-active' : 'nav-button'}
                onClick={() => onTabChange(item.id)}
              >
                <Icon size={17} strokeWidth={2.4} />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>
        <SidebarAccountPanel account={account} />
      </aside>
      <main className="main-surface">
        <div className="license-heartbeat-region">
          {!remote && <UpdateAvailableToast update={update} onOpenDetails={onOpenUpdateDetails} />}
        </div>
        <div className="main-content">{children}</div>
      </main>
    </div>
  );
}

function SidebarAccountPanel({ account }: { account?: ShellAccount | null }) {
  const name = account?.nickname || (account?.confirmed ? '已识别账户' : '等待识别');
  return (
    <div className="sidebar-account-panel">
      <div className="sidebar-account-main">
        <div className="sidebar-account-avatar">
          {account?.avatarUrl ? <FallbackImage src={account.avatarUrl} alt={name} /> : <UserRound size={22} />}
        </div>
        <div className="sidebar-account-text">
          <strong>{name}</strong>
        </div>
      </div>
    </div>
  );
}
