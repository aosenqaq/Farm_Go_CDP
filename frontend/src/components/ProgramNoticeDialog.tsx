import { Megaphone, X } from 'lucide-react';

export type ProgramNoticeDto = {
  content?: string;
  errorCode?: string;
  message?: string;
};

type ProgramNoticeDialogProps = {
  open: boolean;
  loading: boolean;
  notice: ProgramNoticeDto | null;
  onClose: () => void;
  onRetry: () => void;
};

function noticeErrorText(notice: ProgramNoticeDto | null) {
  if (!notice?.errorCode && !notice?.message) return '';
  switch (notice?.errorCode) {
    case 'license_required':
      return '当前未授权，无法获取公告';
    case 'license_notice_unavailable':
      return '公告服务暂不可用';
    case 'license_notice_failed':
      return '公告获取失败，请稍后重试';
    default:
      return notice?.message?.trim() || '公告获取失败';
  }
}

export function ProgramNoticeDialog({ open, loading, notice, onClose, onRetry }: ProgramNoticeDialogProps) {
  if (!open) return null;

  const content = notice?.content?.trim() ?? '';
  const errorMessage = noticeErrorText(notice);

  return (
    <div className="dialog-backdrop program-notice-backdrop" role="presentation" onClick={onClose}>
      <section
        className="program-notice-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="program-notice-title"
        onClick={(event) => event.stopPropagation()}
      >
        <header className="program-notice-header">
          <div className="program-notice-heading">
            <div className="program-notice-icon"><Megaphone size={21} /></div>
            <div>
              <h2 id="program-notice-title">程序公告</h2>
              <p>最新公告</p>
            </div>
          </div>
          <button className="icon-button light" type="button" aria-label="关闭公告" title="关闭" onClick={onClose}>
            <X size={17} />
          </button>
        </header>

        <div className="program-notice-body">
          {loading ? (
            <p className="program-notice-status">正在获取最新公告…</p>
          ) : errorMessage ? (
            <p className="program-notice-status program-notice-error">{errorMessage}</p>
          ) : content ? (
            <pre className="program-notice-content">{content}</pre>
          ) : (
            <p className="program-notice-status">暂无公告内容</p>
          )}
        </div>

        <footer className="program-notice-actions">
          {!loading && (errorMessage || !content) ? (
            <button className="secondary-button" type="button" onClick={onRetry}>重新获取</button>
          ) : null}
          <button className="primary-button" type="button" onClick={onClose}>知道了</button>
        </footer>
      </section>
    </div>
  );
}
