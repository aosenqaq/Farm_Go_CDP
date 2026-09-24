import { Download, RefreshCw, X } from 'lucide-react';

import { getValidatedUpdateDownloadURL, type UpdateStateDto } from '../lib/update';

type UpdateDetailsDialogProps = {
  open: boolean;
  update: UpdateStateDto | null;
  onClose: () => void;
  onDownload: () => void;
};

export function UpdateDetailsDialog({ open, update, onClose, onDownload }: UpdateDetailsDialogProps) {
  if (!open || !update?.available) return null;

  const hasDownload = Boolean(getValidatedUpdateDownloadURL(update));
  const currentVersion = update.current?.name || update.current?.number || '未知';
  const latestVersion = update.latest?.name || update.latest?.number || '未知';

  return (
    <div className="dialog-backdrop update-details-backdrop" role="presentation">
      <section className="update-details-dialog" role="dialog" aria-modal="true" aria-labelledby="update-details-title">
        <header className="update-details-header">
          <div className="update-details-heading">
            <div className="update-details-icon"><RefreshCw size={21} /></div>
            <div>
              <h2 id="update-details-title">发现新版本</h2>
              <p>更新信息已准备就绪</p>
            </div>
          </div>
          <button className="icon-button light" type="button" aria-label="关闭更新详情" title="关闭" onClick={onClose}>
            <X size={17} />
          </button>
        </header>

        <dl className="update-details-versions">
          <div><dt>当前版本</dt><dd>{currentVersion}</dd></div>
          <div><dt>最新版本</dt><dd>{latestVersion}</dd></div>
        </dl>

        {update.latest?.description ? <p className="update-details-description">{update.latest.description}</p> : null}

        <footer className="update-details-actions">
          {hasDownload ? (
            <button className="primary-button" name="open-update-download" type="button" onClick={onDownload}>
              <Download size={17} />
              <span>去下载</span>
            </button>
          ) : null}
          <button className="secondary-button" type="button" onClick={onClose}>稍后处理</button>
        </footer>
      </section>
    </div>
  );
}
