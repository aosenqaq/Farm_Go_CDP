type RecurringUpdateCheckOptions = {
  intervalMinutes: number;
  check: () => Promise<unknown>;
};

export function createSingleFlightUpdateCheck<T>(check: () => Promise<T>) {
  let activeRequest: Promise<T> | null = null;
  return () => {
    if (activeRequest) return activeRequest;
    const request = check();
    activeRequest = request;
    const clearRequest = () => {
      if (activeRequest === request) activeRequest = null;
    };
    void request.then(clearRequest, clearRequest);
    return request;
  };
}

export function startRecurringUpdateChecks(options: RecurringUpdateCheckOptions) {
  const delay = options.intervalMinutes * 60_000;
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const schedule = () => {
    timer = setTimeout(async () => {
      try {
        await options.check();
      } catch {
        // Recurring update checks are intentionally non-blocking.
      } finally {
        if (!stopped) schedule();
      }
    }, delay);
  };

  schedule();
  return () => {
    stopped = true;
    if (timer !== undefined) clearTimeout(timer);
  };
}
