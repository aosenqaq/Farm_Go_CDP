export type PollingController = {
  start(): void;
  refresh(): void;
  stop(): void;
};

export function createPollingController<T>(options: {
  read: (signal: AbortSignal) => Promise<T>;
  apply: (value: T) => void;
  reportError: (error: unknown) => void;
  intervalMs: number;
  shouldStop?: (error: unknown) => boolean;
}): PollingController {
  let timer: ReturnType<typeof setInterval> | undefined;
  let request: AbortController | undefined;
  let generation = 0;
  let running = false;
  let failed = false;

  const stop = () => {
    running = false;
    generation += 1;
    if (timer !== undefined) clearInterval(timer);
    timer = undefined;
    request?.abort();
    request = undefined;
  };

  const refresh = () => {
    if (!running || request) return;
    const requestGeneration = generation;
    request = new AbortController();
    const signal = request.signal;

    void options.read(signal).then((value) => {
      if (running && generation === requestGeneration && !signal.aborted) {
        failed = false;
        options.apply(value);
      }
    }).catch((error) => {
      if (!running || generation !== requestGeneration || signal.aborted) return;
      if (!failed) options.reportError(error);
      failed = true;
      if (options.shouldStop?.(error)) stop();
    }).finally(() => {
      if (generation === requestGeneration) request = undefined;
    });
  };

  return {
    start() {
      if (running) return;
      running = true;
      refresh();
      timer = setInterval(refresh, options.intervalMs);
    },
    refresh,
    stop,
  };
}
