import { describe, expect, it } from 'vitest';

import { createScopedLatestSaveQueue } from './scopedLatestSaveQueue';

type Preferences = { stolenFromMeViewMode: 'timeline' | 'ranking' };

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

function preference(mode: Preferences['stolenFromMeViewMode']): Preferences {
  return { stolenFromMeViewMode: mode };
}

describe('createScopedLatestSaveQueue', () => {
  it('serializes Wails writes so the latest successful preference is final', async () => {
    let currentScope = 'gid:A';
    let parentState = preference('timeline');
    const calls: Array<{ scope: string; value: Preferences; result: Deferred<Preferences> }> = [];
    const queue = createScopedLatestSaveQueue<Preferences>({
      getCurrentScope: () => currentScope,
      persist: (scope, value) => {
        const result = deferred<Preferences>();
        calls.push({ scope, value, result });
        return result.promise;
      },
      onCommitted: (_scope, value) => {
        parentState = value;
      },
    });

    const first = queue.save('gid:A', preference('ranking'));
    const second = queue.save('gid:A', preference('timeline'));
    expect(calls.map((call) => call.value.stolenFromMeViewMode)).toEqual(['ranking']);

    calls[0].result.resolve(preference('ranking'));
    await first;
    expect(calls.map((call) => call.value.stolenFromMeViewMode)).toEqual(['ranking', 'timeline']);
    calls[1].result.resolve(preference('timeline'));
    await second;

    expect(parentState).toEqual(preference('timeline'));
    expect(calls.map((call) => call.scope)).toEqual(['gid:A', 'gid:A']);
  });

  it('rolls parent state back to the last successful write when the latest write fails', async () => {
    let parentState = preference('timeline');
    const calls: Array<Deferred<Preferences>> = [];
    const queue = createScopedLatestSaveQueue<Preferences>({
      getCurrentScope: () => 'gid:A',
      persist: () => {
        const result = deferred<Preferences>();
        calls.push(result);
        return result.promise;
      },
      onCommitted: (_scope, value) => {
        parentState = value;
      },
    });

    const first = queue.save('gid:A', preference('ranking'));
    const second = queue.save('gid:A', preference('timeline'));
    calls[0].resolve(preference('ranking'));
    await first;
    calls[1].reject(new Error('latest save failed'));
    await expect(second).rejects.toThrow('latest save failed');

    expect(parentState).toEqual(preference('ranking'));
  });

  it('cancels queued old-scope writes and ignores old completion after account switch', async () => {
    let currentScope = 'gid:A';
    const parentState = new Map<string, Preferences>();
    const calls: Array<{ scope: string; value: Preferences; result: Deferred<Preferences> }> = [];
    const queue = createScopedLatestSaveQueue<Preferences>({
      getCurrentScope: () => currentScope,
      persist: (scope, value) => {
        const result = deferred<Preferences>();
        calls.push({ scope, value, result });
        return result.promise;
      },
      onCommitted: (scope, value) => {
        parentState.set(scope, value);
      },
    });

    const oldActive = queue.save('gid:A', preference('ranking'));
    const oldQueued = queue.save('gid:A', preference('timeline'));
    const oldQueuedExpectation = expect(oldQueued).rejects.toThrow('superseded');
    currentScope = 'gid:B';
    const newSave = queue.save('gid:B', preference('ranking'));

    calls[0].result.resolve(preference('ranking'));
    await oldActive;
    await oldQueuedExpectation;
    expect(parentState.has('gid:A')).toBe(false);
    expect(calls.map((call) => `${call.scope}:${call.value.stolenFromMeViewMode}`)).toEqual([
      'gid:A:ranking',
      'gid:B:ranking',
    ]);

    calls[1].result.resolve(preference('ranking'));
    await newSave;
    expect(parentState.get('gid:B')).toEqual(preference('ranking'));
  });
});
