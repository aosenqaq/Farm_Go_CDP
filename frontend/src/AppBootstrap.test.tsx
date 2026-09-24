import { act, create } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({ mounts: 0 }));

vi.mock('./AuthorizedApp', async () => {
  const React = await import('react');
  return {
    default: function AuthorizedAppStub() {
      React.useEffect(() => {
        state.mounts += 1;
      }, []);
      return <div>authorized runtime</div>;
    },
    shouldApplyScopedDogGuardResult: () => true,
    shouldRefreshSocialAfterDogGuardAction: () => false,
    socialRefreshPolicyForTab: () => ({}),
  };
});

vi.mock('../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => undefined }));

import App from './App';

describe('App bootstrap', () => {
  it('opens the workspace without a card gate', async () => {
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<App />);
      await Promise.resolve();
    });

    const markup = JSON.stringify(renderer.toJSON());
    expect(markup).toContain('authorized runtime');
    expect(markup).not.toContain('卡密验证');
    expect(state.mounts).toBe(1);
    renderer.unmount();
  });
});
