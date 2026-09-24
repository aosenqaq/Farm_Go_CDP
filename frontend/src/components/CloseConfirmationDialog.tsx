import { Minimize2, Power } from 'lucide-react';
import { useEffect, useState } from 'react';

import { ExitApplication, MinimizeToTray } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

const closeRequestedEvent = 'app:close-requested';

type CloseConfirmationDialogProps = {
  open: boolean;
  onExit: () => void;
  onMinimize: () => void;
  onCancel: () => void;
};

export function CloseConfirmationDialog({ open, onExit, onMinimize, onCancel }: CloseConfirmationDialogProps) {
  if (!open) return null;

  return (
    <div className="dialog-backdrop close-confirmation-backdrop" role="presentation">
      <section className="close-confirmation-dialog" role="dialog" aria-modal="true" aria-labelledby="close-confirmation-title">
        <header className="close-confirmation-header">
          <div className="close-confirmation-icon"><Power size={21} /></div>
          <div>
            <h2 id="close-confirmation-title">关闭 Farm_Go</h2>
            <p>请选择退出程序或继续在后台运行。</p>
          </div>
        </header>
        <footer className="close-confirmation-actions">
          <button name="cancel-close" className="secondary-button" type="button" onClick={onCancel}>取消</button>
          <button name="minimize-to-tray" className="secondary-button close-minimize-button" type="button" onClick={onMinimize}>
            <Minimize2 size={17} />
            <span>最小化到托盘</span>
          </button>
          <button name="exit-application" className="primary-button close-exit-button" type="button" autoFocus onClick={onExit}>
            <Power size={17} />
            <span>退出程序</span>
          </button>
        </footer>
      </section>
    </div>
  );
}

export function CloseConfirmationController() {
  const [open, setOpen] = useState(false);

  useEffect(() => EventsOn(closeRequestedEvent, () => setOpen(true)), []);

  async function minimizeToTray() {
    try {
      await MinimizeToTray();
      setOpen(false);
    } catch (error) {
      console.error(error);
    }
  }

  async function exitApplication() {
    try {
      await ExitApplication();
    } catch (error) {
      console.error(error);
    }
  }

  return <CloseConfirmationDialog open={open} onExit={exitApplication} onMinimize={minimizeToTray} onCancel={() => setOpen(false)} />;
}
