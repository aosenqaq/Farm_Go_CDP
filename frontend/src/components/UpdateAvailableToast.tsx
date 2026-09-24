import { ExternalLink, RefreshCw, X } from 'lucide-react';
import { useEffect, useState } from 'react';

import type { UpdateStateDto } from '../lib/update';

type UpdateAvailableToastProps = {
  update: UpdateStateDto | null;
  onOpenDetails: () => void;
};

export function UpdateAvailableToast({ update, onOpenDetails }: UpdateAvailableToastProps) {
  const [dismissed, setDismissed] = useState(false);
  const releaseKey = `${update?.latest?.number ?? ''}:${update?.latest?.name ?? ''}`;

  useEffect(() => setDismissed(false), [releaseKey]);

  if (!update?.available || dismissed) return null;

  return (
    <div className="update-available-toast" role="status" aria-live="polite">
      <RefreshCw size={17} aria-hidden="true" />
      <div className="update-available-copy">
        <strong>发现新版本 {update.latest?.name || update.latest?.number}</strong>
        <span>当前版本 {update.current?.name || update.current?.number}</span>
      </div>
      <button className="update-toast-action" name="open-update-details" type="button" onClick={onOpenDetails}>
        <ExternalLink size={15} aria-hidden="true" />
        <span>查看更新</span>
      </button>
      <button className="update-toast-dismiss" type="button" aria-label="关闭更新提示" onClick={() => setDismissed(true)}>
        <X size={17} aria-hidden="true" />
      </button>
    </div>
  );
}
