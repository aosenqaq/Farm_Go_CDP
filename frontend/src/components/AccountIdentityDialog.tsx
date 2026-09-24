import { CheckCircle2, Loader2, RefreshCw, UserRound } from 'lucide-react';
import { FallbackImage } from './FallbackImage';

export type RuntimeAccountIdentity = {
  accountKey?: string;
  gid?: number;
  nickname?: string;
  avatarUrl?: string;
  confirmed?: boolean;
  identifiedAt?: string;
  error?: string;
};

type AccountIdentityDialogProps = {
  open: boolean;
  account: RuntimeAccountIdentity | null;
  identifying: boolean;
  confirming: boolean;
  error?: string;
  onConfirm: () => void;
  onRetry: () => void;
};

export function AccountIdentityDialog({
  open,
  account,
  identifying,
  confirming,
  error,
  onConfirm,
  onRetry,
}: AccountIdentityDialogProps) {
  if (!open) return null;

  const canConfirm = Boolean(account?.gid && !account.error && !error);
  const displayName = account?.nickname || (identifying ? '正在识别账户' : '已识别账户');
  const avatarUrl = account?.avatarUrl || '';
  const message = error || account?.error || '';
  const bodyText = identifying && !account ? '正在读取运行账户信息' : '已识别到运行账户，请点击确认';

  return (
    <div className="dialog-backdrop account-gate-backdrop" role="presentation">
      <section className="account-identity-dialog" role="dialog" aria-modal="true" aria-label="运行账户确认">
        <button className="account-identity-refresh" type="button" onClick={onRetry} disabled={identifying}>
          {identifying ? <Loader2 className="spin" size={18} /> : <RefreshCw size={18} />}
          <span>如果识别错误/失败，可以点击这里重新识别</span>
        </button>
        <div className="account-identity-avatar">
          {avatarUrl ? <FallbackImage src={avatarUrl} alt={displayName} /> : <UserRound size={36} />}
        </div>
        <div className="account-identity-copy">
          <h2>{displayName}</h2>
          <p>{bodyText}</p>
          <strong>GId: {account?.gid || '-'}</strong>
        </div>
        {message ? <div className="account-identity-error">{message}</div> : null}
        <footer className="account-identity-actions">
          <button className="primary-button" type="button" onClick={onConfirm} disabled={!canConfirm || confirming || identifying}>
            {confirming ? <Loader2 className="spin" size={17} /> : <CheckCircle2 size={17} />}
            <span>{confirming ? '确认中' : '确认进入'}</span>
          </button>
        </footer>
      </section>
    </div>
  );
}
