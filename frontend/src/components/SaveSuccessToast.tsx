import { CheckCircle2 } from 'lucide-react';

export const SAVE_TOAST_AUTO_DISMISS_MS = 2600;

export function SaveSuccessToast({ visible }: { visible: boolean }) {
  if (!visible) return null;
  return (
    <div className="social-toast ok settings-save-toast" role="status" aria-live="polite">
      <CheckCircle2 size={16} />
      <span>保存成功</span>
    </div>
  );
}
