export type AccountScopeToken = {
  scope: string;
  generation: number;
};

export type AccountScopeGeneration = {
  capture: () => AccountScopeToken;
  currentScope: () => string;
  isCurrent: (token: AccountScopeToken) => boolean;
  renderKey: () => string;
  setScope: (scope: string) => AccountScopeToken;
};

export function createAccountScopeGeneration(initialScope: string): AccountScopeGeneration {
  let scope = initialScope || 'default';
  let generation = 0;

  const capture = () => ({ scope, generation });
  return {
    capture,
    currentScope: () => scope,
    isCurrent: (token) => token.scope === scope && token.generation === generation,
    renderKey: () => `${scope}:${generation}`,
    setScope: (nextScope) => {
      const normalized = nextScope || 'default';
      if (normalized !== scope) {
        scope = normalized;
        generation += 1;
      }
      return capture();
    },
  };
}

type ConfirmableAccount = {
  accountKey?: string;
  confirmed?: boolean;
  error?: string;
};

export function resolveAccountConfirmation<T extends ConfirmableAccount>(current: T | null, result: T) {
  const accepted = result.confirmed === true && Boolean(result.accountKey) && !result.error;
  if (accepted) return { account: result, accepted: true };
  return { account: current?.confirmed ? current : result, accepted: false };
}
