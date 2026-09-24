import { describe, expect, it } from 'vitest';

import { createAccountScopeGeneration, resolveAccountConfirmation } from './accountScope';

type Deferred<T> = {
  promise: Promise<T>;
  resolve: (value: T) => void;
};

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((onResolve) => {
    resolve = onResolve;
  });
  return { promise, resolve };
}

describe('account scope generation', () => {
  for (const oldScope of ['default', 'gid:10001']) {
    it(`ignores a deferred ${oldScope} refresh after confirming account B`, async () => {
      const scope = createAccountScopeGeneration(oldScope);
      const oldToken = scope.capture();
      const oldRefresh = deferred<string>();
      let committed = '';
      const oldCompletion = oldRefresh.promise.then((value) => {
        if (scope.isCurrent(oldToken)) committed = value;
      });

      scope.setScope('gid:10002');
      oldRefresh.resolve('old account state');
      await oldCompletion;
      expect(committed).toBe('');

      const currentToken = scope.capture();
      const currentRefresh = deferred<string>();
      const currentCompletion = currentRefresh.promise.then((value) => {
        if (scope.isCurrent(currentToken)) committed = value;
      });
      currentRefresh.resolve('account B state');
      await currentCompletion;
      expect(committed).toBe('account B state');
    });
  }

  it('keeps the previous confirmed account when a new confirmation fails', () => {
    const previous = { accountKey: 'gid:10001', gid: 10001, confirmed: true, error: '' };
    const failed = { accountKey: 'gid:10002', gid: 10002, confirmed: false, error: 'disk closed' };

    expect(resolveAccountConfirmation(previous, failed)).toEqual({ account: previous, accepted: false });
    expect(resolveAccountConfirmation(null, failed)).toEqual({ account: failed, accepted: false });
  });
});
