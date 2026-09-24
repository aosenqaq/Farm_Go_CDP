import { afterEach, describe, expect, it, vi } from 'vitest';

import { createSingleFlightUpdateCheck, startRecurringUpdateChecks } from './updateCheckScheduler';

type Deferred = {
  promise: Promise<void>;
  resolve: () => void;
};

function deferred(): Deferred {
  let resolve!: () => void;
  const promise = new Promise<void>((onResolve) => {
    resolve = onResolve;
  });
  return { promise, resolve };
}

afterEach(() => vi.useRealTimers());

describe('startRecurringUpdateChecks', () => {
  it('waits for the configured interval before checking', async () => {
    vi.useFakeTimers();
    const check = vi.fn().mockResolvedValue(undefined);
    const stop = startRecurringUpdateChecks({ intervalMinutes: 120, check });

    await vi.advanceTimersByTimeAsync(120 * 60_000 - 1);
    expect(check).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(check).toHaveBeenCalledOnce();

    stop();
  });

  it('starts a fresh interval only after the active check settles', async () => {
    vi.useFakeTimers();
    const pending = deferred();
    const check = vi.fn().mockReturnValueOnce(pending.promise).mockResolvedValue(undefined);
    const stop = startRecurringUpdateChecks({ intervalMinutes: 2, check });

    await vi.advanceTimersByTimeAsync(10 * 60_000);
    expect(check).toHaveBeenCalledOnce();

    pending.resolve();
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(2 * 60_000 - 1);
    expect(check).toHaveBeenCalledOnce();
    await vi.advanceTimersByTimeAsync(1);
    expect(check).toHaveBeenCalledTimes(2);

    stop();
  });

  it('continues after a failed check and stops pending work', async () => {
    vi.useFakeTimers();
    const check = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValue(undefined);
    const stop = startRecurringUpdateChecks({ intervalMinutes: 1, check });

    await vi.advanceTimersByTimeAsync(60_000);
    expect(check).toHaveBeenCalledOnce();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(check).toHaveBeenCalledTimes(2);

    stop();
    await vi.advanceTimersByTimeAsync(10 * 60_000);
    expect(check).toHaveBeenCalledTimes(2);
  });
});

describe('createSingleFlightUpdateCheck', () => {
  it('shares an active request and allows a new request after settlement', async () => {
    const pending = deferred();
    const check = vi.fn().mockReturnValueOnce(pending.promise).mockResolvedValue(undefined);
    const run = createSingleFlightUpdateCheck(check);

    const first = run();
    const second = run();
    expect(second).toBe(first);
    expect(check).toHaveBeenCalledOnce();

    pending.resolve();
    await first;
    await Promise.resolve();

    expect(run()).not.toBe(first);
    expect(check).toHaveBeenCalledTimes(2);
  });
});
