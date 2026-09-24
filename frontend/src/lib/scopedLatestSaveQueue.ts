export type ScopedLatestSaveQueueOptions<T> = {
  getCurrentScope: () => string;
  persist: (scope: string, value: T) => Promise<T>;
  onCommitted: (scope: string, value: T) => void;
};

export type ScopedLatestSaveQueue<T> = {
  save: (scope: string, value: T) => Promise<T>;
  dispose: () => void;
};

type SaveRequest<T> = {
  scope: string;
  value: T;
  resolve: (value: T) => void;
  reject: (reason: unknown) => void;
};

export function createScopedLatestSaveQueue<T>(options: ScopedLatestSaveQueueOptions<T>): ScopedLatestSaveQueue<T> {
  const committedByScope = new Map<string, T>();
  let active = false;
  let disposed = false;
  let pending: SaveRequest<T> | null = null;

  async function drain() {
    if (active || disposed) return;
    active = true;
    try {
      while (pending && !disposed) {
        const request = pending;
        pending = null;
        if (options.getCurrentScope() !== request.scope) {
          request.reject(new Error(`save scope changed before persistence: ${request.scope}`));
          continue;
        }

        try {
          const saved = await options.persist(request.scope, request.value);
          if (disposed) {
            request.reject(new Error('save queue disposed'));
            continue;
          }
          committedByScope.set(request.scope, saved);
          request.resolve(saved);
          if (!pending && options.getCurrentScope() === request.scope) {
            options.onCommitted(request.scope, saved);
          }
        } catch (error) {
          request.reject(error);
          if (!pending && options.getCurrentScope() === request.scope) {
            const committed = committedByScope.get(request.scope);
            if (committed) options.onCommitted(request.scope, committed);
          }
        }
      }
    } finally {
      active = false;
    }
  }

  return {
    save(scope, value) {
      if (disposed) return Promise.reject(new Error('save queue disposed'));
      return new Promise<T>((resolve, reject) => {
        if (pending) {
          pending.reject(new Error('save superseded by a newer request'));
        }
        pending = { scope, value, resolve, reject };
        void drain();
      });
    },
    dispose() {
      disposed = true;
      if (pending) {
        pending.reject(new Error('save queue disposed'));
        pending = null;
      }
    },
  };
}
