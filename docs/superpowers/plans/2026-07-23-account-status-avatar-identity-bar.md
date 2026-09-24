# Account Status Avatar Identity Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show the runtime account avatar in a separate identity bar on the desktop and LAN mobile account-status page while preserving the existing account request and asset behavior.

**Architecture:** Consume the existing `profile.avatarUrl`, `gid`, nickname, and level inside a new page-local `AccountIdentityBar`. Keep experience progress in a separate page-local component, reuse `FallbackImage`, and use scoped CSS plus the existing `.app-shell-remote` 760px breakpoint for the mobile variant.

**Tech Stack:** React 18, TypeScript, Lucide React, Vitest, react-test-renderer, Wails, CSS

---

## File Structure

- Modify `frontend/src/views/AccountStatusView.tsx`: declare the avatar field, render the identity bar, and separate identity from experience progress.
- Modify `frontend/src/views/AccountStatusView.test.tsx`: cover avatar rendering, missing-avatar fallback, refresh recovery, and responsive style contracts.
- Modify `frontend/src/style.css`: add the flat identity-bar styles, simplify the progress band, and add LAN mobile overrides.

No Go files, generated Wails bindings, shared image components, or LAN avatar-proxy files change. The runtime payload and LAN rewrite already provide the required data.

### Task 1: Add Runtime Identity Rendering

**Files:**
- Modify: `frontend/src/views/AccountStatusView.test.tsx`
- Modify: `frontend/src/views/AccountStatusView.tsx`

- [ ] **Step 1: Write failing identity and avatar-state tests**

Add this helper above `describe('AccountStatusView', ...)` in `frontend/src/views/AccountStatusView.test.tsx`:

```tsx
function accountProfileView(avatarUrl?: string) {
  return (
    <AccountStatusView
      initialProfile={{
        gid: 123456789,
        name: 'Dpo.L',
        level: 100,
        avatarUrl,
        levelProgress: { current: 161475, needed: 401000, remaining: 239525, percent: 40, nextLevel: 101 },
      }}
      initialFertilizer={{}}
      initialWarehouse={{ status: 'runtime', items: [] }}
    />
  );
}
```

Add these tests inside the existing `describe` block:

```tsx
it('renders the runtime avatar and separated account identity', () => {
  const html = renderToStaticMarkup(accountProfileView('https://example.test/runtime-avatar.png'));

  expect(html).toContain('class="account-identity-bar"');
  expect(html).toContain('class="account-identity-avatar"');
  expect(html).toContain('src="https://example.test/runtime-avatar.png"');
  expect(html).toContain('alt="Dpo.L"');
  expect(html).toContain('GID 123,456,789');
  expect(html).toContain('Lv. 100');
  expect(html).toContain('经验升级进度');
});

it('shows the user icon when the runtime profile has no avatar', () => {
  let tree: ReturnType<typeof create>;
  act(() => {
    tree = create(accountProfileView());
  });

  const avatar = tree!.root.findByProps({ className: 'account-identity-avatar' });
  expect(avatar.findAllByType('img')).toHaveLength(0);
  expect(avatar.findAll((node) => typeof node.props.className === 'string' && node.props.className.includes('lucide-user-round'))).toHaveLength(1);
});

it('retries with a refreshed avatar URL after the previous image failed', () => {
  let tree: ReturnType<typeof create>;
  act(() => {
    tree = create(accountProfileView('https://example.test/first-avatar.png'));
  });

  const currentAvatar = () => tree!.root.findAllByType('img').find((node) => node.props.alt === 'Dpo.L')!;
  act(() => currentAvatar().props.onError());
  expect(currentAvatar().props.src).toBe('/logo.png');

  act(() => {
    tree!.update(accountProfileView('https://example.test/refreshed-avatar.png'));
  });
  expect(currentAvatar().props.src).toBe('https://example.test/refreshed-avatar.png');
});
```

- [ ] **Step 2: Run the focused test and verify failure**

Run from `frontend`:

```powershell
npm test -- src/views/AccountStatusView.test.tsx
```

Expected: FAIL because `account-identity-bar` does not exist and the profile type/render path does not yet consume `avatarUrl`.

- [ ] **Step 3: Add the avatar field, imports, identity bar, and progress-only component**

In `frontend/src/views/AccountStatusView.tsx`, add `UserRound` to the Lucide import and add the shared image import:

```tsx
import { FallbackImage } from '../components/FallbackImage';
```

Add this field to `AccountProfileLike`:

```tsx
avatarUrl?: string;
```

Replace the current growth-card call in the account-assets section with:

```tsx
<AccountIdentityBar profile={profile} />
<AccountGrowthProgress profile={profile} />
```

Replace the existing `AccountGrowthCard` function with these two page-local components:

```tsx
function AccountIdentityBar({ profile }: { profile?: AccountProfileLike }) {
  const name = profile?.name || profile?.nick || '-';
  const avatarUrl = profile?.avatarUrl?.trim() || '';
  const gid = Number(profile?.gid) > 0 ? formatNumber(profile?.gid) : '-';
  const level = profile?.level ?? '-';

  return (
    <div className="account-identity-bar">
      <div className="account-identity-avatar">
        {avatarUrl
          ? <FallbackImage key={avatarUrl} src={avatarUrl} alt={name} />
          : <UserRound size={24} aria-hidden="true" />}
      </div>
      <div className="account-identity-copy">
        <strong className="account-identity-name" title={name}>{name}</strong>
        <span className="account-identity-gid">GID {gid}</span>
      </div>
      <span className="account-identity-level">Lv. {level}</span>
    </div>
  );
}

function AccountGrowthProgress({ profile }: { profile?: AccountProfileLike }) {
  const progress = normalizeLevelProgress(profile);
  if (!progress) {
    return (
      <div className="account-growth-card account-growth-unavailable">
        <p>经验数据暂不可用</p>
      </div>
    );
  }
  return (
    <div className="account-growth-card">
      <div className="account-growth-details">
        <div className="account-growth-heading"><span>经验升级进度</span><strong>{formatNumber(progress.current)} / {formatNumber(progress.needed)}</strong></div>
        <div className="account-growth-progress" role="progressbar" aria-label="经验升级进度" aria-valuemin={0} aria-valuemax={progress.needed} aria-valuenow={progress.current}>
          <span style={{ width: `${progress.percent}%` }} />
        </div>
        <div className="account-growth-footer"><span>下一等级 Lv. {progress.nextLevel}</span><span>还差 {formatNumber(progress.remaining)}</span></div>
      </div>
    </div>
  );
}
```

The `key={avatarUrl}` is required: it remounts `FallbackImage` after a refresh changes the URL, clearing the component's internal failed state without changing the shared component.

- [ ] **Step 4: Run the focused test and verify success**

Run from `frontend`:

```powershell
npm test -- src/views/AccountStatusView.test.tsx
```

Expected: PASS for all `AccountStatusView` tests.

- [ ] **Step 5: Commit identity behavior**

```powershell
git add -- frontend/src/views/AccountStatusView.tsx frontend/src/views/AccountStatusView.test.tsx
git commit -m "feat: show runtime account identity"
```

### Task 2: Apply Desktop and LAN Mobile Layout

**Files:**
- Modify: `frontend/src/views/AccountStatusView.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write the failing responsive style contract test**

Add this test inside the existing `describe` block in `frontend/src/views/AccountStatusView.test.tsx`:

```tsx
it('uses balanced account identity sizing on desktop and LAN mobile', () => {
  const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
  const mobileStyles = mediaDeclarations(css, 760);
  const identityBar = ruleDeclarations(css, '.account-identity-bar');
  const desktopAvatar = ruleDeclarations(css, '.account-identity-avatar');
  const identityName = ruleDeclarations(css, '.account-identity-name,');
  const mobileIdentityBar = ruleDeclarations(mobileStyles, '.app-shell-remote .account-identity-bar');
  const mobileAvatar = ruleDeclarations(mobileStyles, '.app-shell-remote .account-identity-avatar');
  const mobileMetrics = ruleDeclarations(mobileStyles, '.app-shell-remote .account-metric-grid');

  expect(identityBar).toMatch(/grid-template-columns:\s*52px\s+minmax\(0,\s*1fr\)\s+auto;/);
  expect(identityBar).toMatch(/box-shadow:\s*none;/);
  expect(desktopAvatar).toMatch(/width:\s*52px;/);
  expect(desktopAvatar).toMatch(/height:\s*52px;/);
  expect(identityName).toMatch(/overflow:\s*hidden;/);
  expect(identityName).toMatch(/text-overflow:\s*ellipsis;/);
  expect(identityName).toMatch(/white-space:\s*nowrap;/);
  expect(mobileIdentityBar).toMatch(/grid-template-columns:\s*44px\s+minmax\(0,\s*1fr\)\s+auto;/);
  expect(mobileAvatar).toMatch(/width:\s*44px;/);
  expect(mobileAvatar).toMatch(/height:\s*44px;/);
  expect(mobileMetrics).toMatch(/grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/);
});
```

- [ ] **Step 2: Run the focused test and verify failure**

Run from `frontend`:

```powershell
npm test -- src/views/AccountStatusView.test.tsx
```

Expected: FAIL because the new identity selectors and 52px/44px responsive dimensions are absent.

- [ ] **Step 3: Add base identity styles and simplify the progress band**

In the account-status section of `frontend/src/style.css`, add:

```css
.account-identity-bar {
  display: grid;
  grid-template-columns: 52px minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  min-width: 0;
  padding: 12px 14px;
  background: #f1f5e8;
  box-shadow: none;
}

.account-identity-avatar {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  overflow: hidden;
  border: 2px solid #fffdf7;
  border-radius: 50%;
  color: #4c544b;
  background: #e7ebdf;
}

.account-identity-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.account-identity-copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.account-identity-name,
.account-identity-gid {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-identity-name {
  color: #172015;
  font-size: 18px;
}

.account-identity-gid {
  color: #68715f;
  font-size: 12px;
}

.account-identity-level {
  padding: 5px 8px;
  border-radius: 5px;
  color: #2f8d35;
  background: #fffdf7;
  font-size: 13px;
  font-weight: 900;
  white-space: nowrap;
}
```

Replace the existing `.account-growth-card` declarations with:

```css
.account-growth-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-items: center;
  min-height: 82px;
  padding: 13px 16px;
  border-left: 4px solid #d6ff67;
  background: #f1f5e8;
}
```

Delete the obsolete `.account-growth-identity`, `.account-growth-level`, `.account-growth-identity strong`, and `.account-growth-unavailable` grid-column rules. Keep `.account-growth-details`, heading, progress, footer, and unavailable paragraph declarations. Remove the obsolete `.account-growth-card` grid override from the existing `@media (max-width: 980px)` block.

- [ ] **Step 4: Add LAN mobile identity and asset-grid overrides**

Inside the existing `@media (max-width: 760px)` block in `frontend/src/style.css`, add:

```css
.app-shell-remote .account-section {
  gap: 12px;
  padding: 12px;
}

.app-shell-remote .account-identity-bar {
  grid-template-columns: 44px minmax(0, 1fr) auto;
  gap: 10px;
  padding: 10px 12px;
}

.app-shell-remote .account-identity-avatar {
  width: 44px;
  height: 44px;
}

.app-shell-remote .account-identity-name {
  font-size: 16px;
}

.app-shell-remote .account-identity-level {
  padding: 4px 7px;
  font-size: 12px;
}

.app-shell-remote .account-growth-card {
  min-height: 0;
  padding: 12px;
}

.app-shell-remote .account-metric-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
```

- [ ] **Step 5: Run the focused test and verify success**

Run from `frontend`:

```powershell
npm test -- src/views/AccountStatusView.test.tsx
```

Expected: PASS, including the desktop 52px, LAN mobile 44px, and two-column mobile asset assertions.

- [ ] **Step 6: Commit responsive styling**

```powershell
git add -- frontend/src/style.css frontend/src/views/AccountStatusView.test.tsx
git commit -m "style: separate account identity and progress"
```

### Task 3: Run Regression and Build Verification

**Files:**
- Verify: `frontend/src/views/AccountStatusView.tsx`
- Verify: `frontend/src/views/AccountStatusView.test.tsx`
- Verify: `frontend/src/style.css`
- Verify unchanged: `internal/farm/account_status.go`
- Verify unchanged: `internal/lanaccess/avatar_proxy.go`

- [ ] **Step 1: Run the focused frontend test once more from a clean component state**

Run from `frontend`:

```powershell
npm test -- src/views/AccountStatusView.test.tsx
```

Expected: PASS with no failed `AccountStatusView` tests.

- [ ] **Step 2: Run the complete frontend test suite**

Run from `frontend`:

```powershell
npm test
```

Expected: PASS with zero failed test files.

- [ ] **Step 3: Run the production frontend build**

Run from `frontend`:

```powershell
npm run build
```

Expected: TypeScript and Vite complete successfully and write the ignored `frontend/dist` output.

- [ ] **Step 4: Run the unchanged account and LAN avatar backend regressions**

Run from the repository root:

```powershell
go test ./internal/farm ./internal/lanaccess
```

Expected: both packages report `ok`, including account-status and LAN avatar-proxy tests.

- [ ] **Step 5: Check formatting and final worktree scope**

Run from the repository root:

```powershell
git diff --check HEAD
git status --short
```

Expected: `git diff --check HEAD` prints nothing. The two implementation commits contain only `AccountStatusView.tsx`, `AccountStatusView.test.tsx`, and `style.css`; the pre-existing `resources/gameConfig.bundle.zip` modification remains unstaged and unchanged.
