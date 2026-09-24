import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { Eye, EyeOff, KeyRound, Loader2, ShieldCheck } from 'lucide-react';

const CROP_RADIANCE_IMAGES = [
  '/crops/license-gate/white-radish.png',
  '/crops/license-gate/carrot.png',
  '/crops/license-gate/napa-cabbage.png',
  '/crops/license-gate/garlic.png',
  '/crops/license-gate/wheat.png',
  '/crops/license-gate/corn.png',
  '/crops/license-gate/carrot.png',
  '/crops/license-gate/napa-cabbage.png',
] as const;

export type CredentialGateProps = {
  credential: string;
  loading: boolean;
  error: string;
  headerLabel: string;
  title: string;
  description: string;
  fieldLabel: string;
  submitLabel: string;
  inputId: string;
  onSubmit: (credential: string) => void | Promise<void>;
  onCredentialChange?: (credential: string) => void;
  remember?: {
    checked: boolean;
    label: string;
    onChange: (checked: boolean) => void;
  };
  feedbackAction?: ReactNode;
  supplementary?: ReactNode;
};

export function CredentialGate({
  credential: initialCredential,
  loading,
  error,
  headerLabel,
  title,
  description,
  fieldLabel,
  submitLabel,
  inputId,
  onSubmit,
  onCredentialChange,
  remember,
  feedbackAction,
  supplementary,
}: CredentialGateProps) {
  const [credential, setCredential] = useState(initialCredential);
  const [visible, setVisible] = useState(false);
  const canSubmit = credential.trim().length > 0 && !loading;

  useEffect(() => {
    setCredential(initialCredential);
  }, [initialCredential]);

  function updateCredential(value: string) {
    setCredential(value);
    onCredentialChange?.(value);
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (canSubmit) void onSubmit(credential.trim());
  }

  return (
    <main className="license-gate" aria-busy={loading}>
      <div className="license-crop-radiance" aria-hidden="true">
        {CROP_RADIANCE_IMAGES.map((src, index) => (
          <img className="license-crop" src={src} alt="" key={`${src}-${index}`} />
        ))}
      </div>
      <header className="license-gate-bar">
        <span className="license-gate-brand"><ShieldCheck size={22} /> Farm Go</span>
        <span>{headerLabel}</span>
      </header>
      <form className="license-gate-form" onSubmit={submit}>
        <div className="license-gate-title">
          <span className="license-gate-icon"><KeyRound size={22} /></span>
          <div><h1>{title}</h1><p>{description}</p></div>
        </div>
        <label htmlFor={inputId}>{fieldLabel}</label>
        <div className="license-card-input">
          <input
            id={inputId}
            type={visible ? 'text' : 'password'}
            value={credential}
            onChange={(event) => updateCredential(event.target.value)}
            autoComplete="current-password"
            disabled={loading}
          />
          <button type="button" className="license-visibility" onClick={() => setVisible((current) => !current)} aria-label={visible ? `隐藏${fieldLabel}` : `显示${fieldLabel}`}>
            {visible ? <EyeOff size={18} /> : <Eye size={18} />}
          </button>
        </div>
        {remember && (
          <label className="license-remember"><input type="checkbox" checked={remember.checked} onChange={(event) => remember.onChange(event.target.checked)} disabled={loading} /> {remember.label}</label>
        )}
        <div className="license-feedback-row">
          <p className="license-feedback" role="status">{error || (loading ? '正在验证授权状态...' : '')}</p>
          {feedbackAction}
        </div>
        <button className="license-submit" type="submit" disabled={!canSubmit}>
          {loading ? <Loader2 className="spin" size={18} /> : <ShieldCheck size={18} />} {submitLabel}
        </button>
      </form>
      {supplementary}
    </main>
  );
}
