import { afterEach, describe, expect, it, vi } from 'vitest';

import { createPollingController } from './pollingController';

type Deferred<T> = {
  promise: Promise<T>;
  resolve: (value: T) => void;
  reject: (reason: unknown) => void;
};

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((onResolve, onReject) => {
    resolve = onResolve;
    reject = onReject;
  });
  return { promise, resolve, reject };
}

afterEach(() => vi.useRealTimers());

describe('createPollingController', () => {
  it('reads immediately when started', async () => {
    vi.useFakeTimers();
    const read = vi.fn().mockResolvedValue('initial');
    const apply = vi.fn();
    const controller = createPollingController({
      read,
      apply,
      reportError: vi.fn(),
      intervalMs: 1_000,
    });

    controller.start();
    expect(read).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(0);
    expect(apply).toHaveBeenCalledWith('initial');

    controller.stop();
  });

  it('treats duplicate starts as a no-op', async () => {
    vi.useFakeTimers();
    const read = vi.fn().mockResolvedValue('value');
    const controller = createPollingController({
      read,
      apply: vi.fn(),
      reportError: vi.fn(),
      intervalMs: 1_000,
    });

    controller.start();
    controller.start();
    await vi.advanceTimersByTimeAsync(1_000);

    expect(read).toHaveBeenCalledTimes(2);
    controller.stop();
  });

  it('allows only one request at a time', async () => {
    vi.useFakeTimers();
    const first = deferred<string>();
    const read = vi.fn().mockReturnValueOnce(first.promise).mockResolvedValue('next');
    const controller = createPollingController({
      read,
      apply: vi.fn(),
      reportError: vi.fn(),
      intervalMs: 1_000,
    });

    controller.start();
    await vi.advanceTimersByTimeAsync(3_000);
    expect(read).toHaveBeenCalledTimes(1);

    first.resolve('first');
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(1_000);
    expect(read).toHaveBeenCalledTimes(2);

    controller.stop();
  });

  it('aborts the active request and suppresses a late result after stop', async () => {
    vi.useFakeTimers();
    const pending = deferred<string>();
    let signal: AbortSignal | undefined;
    const read = vi.fn((activeSignal: AbortSignal) => {
      signal = activeSignal;
      return pending.promise;
    });
    const apply = vi.fn();
    const reportError = vi.fn();
    const controller = createPollingController({ read, apply, reportError, intervalMs: 1_000 });

    controller.start();
    controller.stop();
    expect(signal?.aborted).toBe(true);

    pending.resolve('late');
    await vi.advanceTimersByTimeAsync(3_000);
    expect(apply).not.toHaveBeenCalled();
    expect(reportError).not.toHaveBeenCalled();
    expect(read).toHaveBeenCalledTimes(1);
  });

  it('reports once per failure transition and resets after success', async () => {
    vi.useFakeTimers();
    const firstError = new Error('first');
    const repeatedError = new Error('repeated');
    const laterError = new Error('later');
    const read = vi.fn()
      .mockRejectedValueOnce(firstError)
      .mockRejectedValueOnce(repeatedError)
      .mockResolvedValueOnce('recovered')
      .mockRejectedValueOnce(laterError);
    const apply = vi.fn();
    const reportError = vi.fn();
    const controller = createPollingController({ read, apply, reportError, intervalMs: 1_000 });

    controller.start();
    await vi.advanceTimersByTimeAsync(3_000);

    expect(read).toHaveBeenCalledTimes(4);
    expect(apply).toHaveBeenCalledWith('recovered');
    expect(reportError).toHaveBeenCalledTimes(2);
    expect(reportError).toHaveBeenNthCalledWith(1, firstError);
    expect(reportError).toHaveBeenNthCalledWith(2, laterError);

    controller.stop();
  });

  it('stops polling when shouldStop accepts an error', async () => {
    vi.useFakeTimers();
    const terminalError = new Error('unauthorized');
    let signal: AbortSignal | undefined;
    const read = vi.fn((activeSignal: AbortSignal) => {
      signal = activeSignal;
      return Promise.reject(terminalError);
    });
    const reportError = vi.fn();
    const shouldStop = vi.fn((error: unknown) => error === terminalError);
    const controller = createPollingController({
      read,
      apply: vi.fn(),
      reportError,
      intervalMs: 1_000,
      shouldStop,
    });

    controller.start();
    await vi.advanceTimersByTimeAsync(5_000);

    expect(reportError).toHaveBeenCalledOnce();
    expect(shouldStop).toHaveBeenCalledWith(terminalError);
    expect(signal?.aborted).toBe(true);
    expect(read).toHaveBeenCalledOnce();
  });
});
