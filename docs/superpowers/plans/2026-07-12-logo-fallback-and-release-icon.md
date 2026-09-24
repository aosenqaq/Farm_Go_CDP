# Logo Fallback and Release Icon Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Use the supplied logo as the Windows release icon and the fallback image for every frontend image load failure.

**Architecture:** A small React image component owns the one-shot error transition from a requested image URL to `/logo.png`. Existing image call sites use this component without changing their current visibility conditions, classes, or alt text. Wails continues using its standard `build/appicon.png` and `build/windows/icon.ico` files.

**Tech Stack:** React 18, TypeScript, Vitest, react-test-renderer, Vite, Wails, Windows ICO assets.

---

### Task 1: Shared fallback image component

**Files:**
- Create: `frontend/src/components/FallbackImage.tsx`
- Create: `frontend/src/components/FallbackImage.test.tsx`
- Create: `frontend/public/logo.png`

- [ ] **Step 1: Write the failing component test**

```tsx
it('uses the bundled logo once when the requested image fails', () => {
  let renderer: ReturnType<typeof create>;
  act(() => {
    renderer = create(<FallbackImage alt="avatar" src="https://example.test/avatar.png" />);
  });

  const image = renderer!.root.findByType('img');
  expect(image.props.src).toBe('https://example.test/avatar.png');

  act(() => image.props.onError());

  const fallback = renderer!.root.findByType('img');
  expect(fallback.props.src).toBe('/logo.png');
  expect(fallback.props.onError).toBeUndefined();
});
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `npm test -- FallbackImage.test.tsx`

Expected: FAIL because `FallbackImage` does not exist.

- [ ] **Step 3: Copy the supplied source asset and implement the component**

Copy `logo/logo.png` to `frontend/public/logo.png`, then add:

```tsx
import { useState, type ComponentPropsWithoutRef } from 'react';

type FallbackImageProps = Omit<ComponentPropsWithoutRef<'img'>, 'onError' | 'src'> & {
  src: string;
};

export function FallbackImage({ src, ...props }: FallbackImageProps) {
  const [failed, setFailed] = useState(false);

  return <img {...props} src={failed ? '/logo.png' : src} onError={failed ? undefined : () => setFailed(true)} />;
}
```

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `npm test -- FallbackImage.test.tsx`

Expected: PASS.

### Task 2: Route every frontend image through the fallback component

**Files:**
- Modify: `frontend/src/components/AccountIdentityDialog.tsx`
- Modify: `frontend/src/components/AppShell.tsx`
- Modify: `frontend/src/views/AssetsLandView.tsx`
- Modify: `frontend/src/views/AutomationView.tsx`

- [ ] **Step 1: Replace the seven image elements with `FallbackImage`**

Add the component import to each file and replace these expressions while preserving their props:

```tsx
<FallbackImage alt={displayName} src={avatarUrl} />
<FallbackImage alt={name} src={account.avatarUrl} />
<FallbackImage alt={landDisplayName(land)} src={imageUrl} />
<FallbackImage className="land-mutation-icon" alt={land.mutationLabel || '变异'} src={iconUrl} />
<FallbackImage alt="" src={item.imageUrl} />
<FallbackImage alt="" src={option.imageUrl} />
```

Use the mutation-icon expression at both existing locations in `AssetsLandView.tsx`.

- [ ] **Step 2: Type-check and run the focused image tests**

Run: `npm test -- FallbackImage.test.tsx AccountIdentityDialog.test.tsx AppShell.test.tsx`

Expected: PASS.

### Task 3: Replace the Wails Windows icon assets

**Files:**
- Modify: `build/appicon.png`
- Modify: `build/windows/icon.ico`

- [ ] **Step 1: Update the PNG source asset**

Copy `logo/logo.png` over `build/appicon.png`.

- [ ] **Step 2: Generate the multi-size Windows ICO**

Generate `build/windows/icon.ico` from `logo/logo.png` with 16, 24, 32, 48, 64, 128, and 256 pixel entries. The ICO must be valid Windows icon data rather than a PNG renamed to `.ico`.

- [ ] **Step 3: Inspect the generated icon**

Run an image metadata tool against `build/windows/icon.ico`.

Expected: a readable ICO containing multiple square icon sizes, including 256x256.

### Task 4: Build verification

**Files:**
- Verify only.

- [ ] **Step 1: Run all frontend tests**

Run: `npm test`

Expected: PASS with no test failures.

- [ ] **Step 2: Build the production frontend**

Run: `npm run build`

Expected: TypeScript and Vite complete successfully.

- [ ] **Step 3: Build the Wails Windows application**

Run: `wails build -platform windows/amd64`

Expected: the Windows executable and its installer use `build/windows/icon.ico`; inspect the output file and report its path.
