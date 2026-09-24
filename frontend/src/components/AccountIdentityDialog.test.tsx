import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { AccountIdentityDialog, type RuntimeAccountIdentity } from './AccountIdentityDialog';

describe('AccountIdentityDialog', () => {
  it('shows the identified account and confirmation copy', () => {
    const account: RuntimeAccountIdentity = {
      accountKey: 'gid:123456',
      gid: 123456,
      nickname: 'Dpo.L',
      avatarUrl: 'https://example.test/avatar.png',
      confirmed: false,
    };

    const html = renderToStaticMarkup(
      <AccountIdentityDialog open account={account} confirming={false} identifying={false} onConfirm={() => undefined} onRetry={() => undefined} />,
    );

    expect(html).toContain('已识别到运行账户，请点击确认');
    expect(html).toContain('Dpo.L');
    expect(html).toContain('GId');
    expect(html).toContain('123456');
    expect(html).toContain('https://example.test/avatar.png');
    expect(html).toContain('如果识别错误/失败，可以点击这里重新识别');
    expect(html).toContain('class="account-identity-refresh"');
    expect(html).toContain('lucide-refresh-cw');
  });

  it('shows retry feedback when identity lookup fails', () => {
    const html = renderToStaticMarkup(
      <AccountIdentityDialog open account={null} error="未识别到有效 GId" confirming={false} identifying={false} onConfirm={() => undefined} onRetry={() => undefined} />,
    );

    expect(html).toContain('未识别到有效 GId');
    expect(html).toContain('如果识别错误/失败，可以点击这里重新识别');
    expect(html).not.toContain('secondary-button');
  });

  it('disables the refresh banner while identifying', () => {
    const html = renderToStaticMarkup(
      <AccountIdentityDialog open account={null} confirming={false} identifying onConfirm={() => undefined} onRetry={() => undefined} />,
    );

    expect(html).toMatch(/<button class="account-identity-refresh"[^>]*disabled=""/);
    expect(html).toContain('spin');
    expect(html).toContain('如果识别错误/失败，可以点击这里重新识别');
  });
});
