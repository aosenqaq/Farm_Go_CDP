import { useEffect, useState } from 'react';

import AuthorizedApp from './AuthorizedApp';
import { CredentialGate } from './LicenseGate';
import { syncRemoteSession, type RemoteBridgeWindow } from './lib/remoteBridge';

const loginFailureMessage = '访问密码验证失败，请重试';
const tunnelHTTPSMessage = '安全隧道必须通过 HTTPS FRP 入口访问';

function insecureTunnelPage() {
  if (typeof window === 'undefined') return false;
  const remoteWindow = window as RemoteBridgeWindow;
  return remoteWindow.__FARM_GO_TUNNEL__ === true && window.location.protocol !== 'https:';
}

function RemoteApp() {
  const requiresTunnelHTTPS = insecureTunnelPage();
  const [sessionChecked, setSessionChecked] = useState(false);
  const [authorized, setAuthorized] = useState(false);
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    void (async () => {
      try {
        const session = await syncRemoteSession();
        if (!session.authorized) return;
        if (!active) return;
        setAuthorized(true);
      } catch {
        if (active) setError('无法验证访问状态，请稍后重试');
      } finally {
        if (active) setSessionChecked(true);
      }
    })();
    return () => {
      active = false;
    };
  }, []);

  async function submit(nextPassword: string) {
    if (requiresTunnelHTTPS) {
      setAuthorized(false);
      setError(tunnelHTTPSMessage);
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: nextPassword }),
      });
      if (!response.ok) throw new Error(`Remote login failed: ${response.status}`);

      const session = await syncRemoteSession();
      if (!session.authorized || !session.csrfToken) throw new Error('Remote session was not established');
      setAuthorized(true);
    } catch {
      setAuthorized(false);
      setError(loginFailureMessage);
    } finally {
      setSubmitting(false);
    }
  }

  if (sessionChecked && authorized) return <AuthorizedApp remote />;

  return (
    <CredentialGate
      credential={password}
      loading={!sessionChecked || submitting}
      error={requiresTunnelHTTPS ? tunnelHTTPSMessage : error}
      headerLabel="访问密码验证"
      title="验证访问密码"
      description="请输入访问密码以继续使用。"
      fieldLabel="访问密码"
      submitLabel="进入控制台"
      inputId="remote-password"
      onCredentialChange={setPassword}
      onSubmit={submit}
    />
  );
}

export default RemoteApp;
