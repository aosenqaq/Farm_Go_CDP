import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { CredentialGate } from './LicenseGate';

describe('CredentialGate', () => {
  it('renders an access-password form without a remembered card option', () => {
    const html = renderToStaticMarkup(
      <CredentialGate
        credential=""
        loading={false}
        error=""
        headerLabel="访问密码验证"
        title="验证访问密码"
        description="请输入访问密码以继续使用。"
        fieldLabel="访问密码"
        submitLabel="进入控制台"
        inputId="remote-password"
        onSubmit={() => undefined}
      />,
    );

    expect(html).toContain('验证访问密码');
    expect(html).toContain('访问密码');
    expect(html).toContain('进入控制台');
    expect(html).not.toContain('在此设备记住卡密');
    expect(html).not.toContain('卡密验证');
  });
});
