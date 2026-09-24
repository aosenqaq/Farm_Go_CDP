type RemoteFetch = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;

export type RemoteSession = {
  authorized: boolean;
  csrfToken: string;
};

type RemoteAppBridge = Record<string, (...args: unknown[]) => Promise<unknown>>;

export type RemoteBridgeWindow = {
  __FARM_GO_REMOTE__?: boolean;
  __FARM_GO_TUNNEL__?: boolean;
  go?: {
    desktop?: {
      App?: RemoteAppBridge;
    };
  };
};

type InstallRemoteBridgeOptions = {
  window?: RemoteBridgeWindow;
  fetchImpl?: RemoteFetch;
  csrfToken?: string;
};

let csrfToken = '';
let sessionRequest: Promise<RemoteSession> | null = null;

function remoteFetchError(action: string, response: Response) {
  return new Error(`${action} failed: ${response.status}${response.statusText ? ` ${response.statusText}` : ''}`);
}

function browserWindow() {
  return window as RemoteBridgeWindow;
}

export async function syncRemoteSession(fetchImpl: RemoteFetch = fetch): Promise<RemoteSession> {
  if (!sessionRequest) {
    sessionRequest = (async () => {
      const response = await fetchImpl('/api/auth/session', { credentials: 'same-origin' });
      if (!response.ok) throw remoteFetchError('Remote session request', response);
      const payload: unknown = await response.json();
      const source = payload && typeof payload === 'object' ? payload as Record<string, unknown> : {};
      const session = {
        authorized: source.authorized === true,
        csrfToken: typeof source.csrfToken === 'string' ? source.csrfToken : '',
      };
      csrfToken = session.authorized ? session.csrfToken : '';
      return session;
    })();
  }

  try {
    return await sessionRequest;
  } finally {
    sessionRequest = null;
  }
}

export function installRemoteBridge(options: InstallRemoteBridgeOptions = {}) {
  const targetWindow = options.window || browserWindow();
  if (targetWindow.__FARM_GO_REMOTE__ !== true) return;

  if (typeof options.csrfToken === 'string') csrfToken = options.csrfToken;
  const fetchImpl = options.fetchImpl || fetch;
  const app = new Proxy({} as RemoteAppBridge, {
    get(_target, method) {
      if (typeof method !== 'string' || method === 'then') return undefined;
      return async (...args: unknown[]) => {
        if (!csrfToken) {
          const session = await syncRemoteSession(fetchImpl);
          if (!session.authorized || !session.csrfToken) throw new Error('Remote session is not authorized');
        }
        const response = await fetchImpl(`/api/rpc/${encodeURIComponent(method)}`, {
          method: 'POST',
          credentials: 'same-origin',
          headers: {
            'Content-Type': 'application/json',
            'X-Farm-Go-CSRF': csrfToken,
          },
          body: JSON.stringify({ args }),
        });
        if (!response.ok) throw remoteFetchError(`Remote RPC ${method}`, response);
        return response.json();
      };
    },
  });

  targetWindow.go ||= {};
  targetWindow.go.desktop ||= {};
  targetWindow.go.desktop.App = app;
}
