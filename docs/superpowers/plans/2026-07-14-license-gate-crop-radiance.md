# License Gate Crop Radiance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework the Farm Go card-key gate into a centered, high-contrast authorization card with a subtle animated radiance of packaged crop assets.

**Architecture:** `LicenseGate.tsx` remains the owner of all existing card-input interactions and renders one additional non-interactive decorative crop layer. `style.css` owns the layout, crop trajectories, focus contrast, reduced-motion fallback, and responsive rules. A fixed set of existing mature crop PNGs is copied under the frontend public directory so Vite/Wails serves stable runtime asset URLs.

**Tech Stack:** React 18, TypeScript, CSS keyframe animation, Vite public assets, Vitest, react-test-renderer, lucide-react.

---

## File Structure

- Create: `frontend/public/crops/license-gate/lilac.png` - mature lilac crop background asset.
- Create: `frontend/public/crops/license-gate/ginseng.png` - mature ginseng crop background asset.
- Create: `frontend/public/crops/license-gate/loofah.png` - fruiting loofah crop background asset.
- Create: `frontend/public/crops/license-gate/orange-jasmine.png` - mature orange jasmine crop background asset.
- Create: `frontend/public/crops/license-gate/ginseng-fruit.png` - mature ginseng-fruit crop background asset.
- Modify: `frontend/src/LicenseGate.tsx` - render the decorative background layer while preserving existing form behavior.
- Modify: `frontend/src/LicenseGate.test.tsx` - protect the decorative layer accessibility contract and current gate controls.
- Modify: `frontend/src/style.css` - implement centered gate, high-contrast input, animation, reduced-motion, and narrow-screen layout.

### Task 1: Add The Decorative Layer Contract

**Files:**
- Modify: `frontend/src/LicenseGate.test.tsx`

- [ ] **Step 1: Write the failing component tests**

Add these tests after the existing Chinese-rendering test. They assert that crop images are present only as a decorative layer and remain inaccessible to assistive technology.

```tsx
it('renders the crop radiance as a decorative, hidden background layer', () => {
  const html = renderToStaticMarkup(
    <LicenseGate card="" remembered={false} loading={false} error="" onSubmit={() => undefined} />,
  );

  expect(html).toContain('class="license-crop-radiance"');
  expect(html).toContain('aria-hidden="true"');
  expect(html).toContain('src="/crops/license-gate/lilac.png"');
  expect(html).toContain('src="/crops/license-gate/loofah.png"');
  expect(html).toContain('alt=""');
});

it('keeps the interactive card controls outside the decorative crop layer', () => {
  const html = renderToStaticMarkup(
    <LicenseGate card="card-123" remembered={true} loading={false} error="" onSubmit={() => undefined} />,
  );

  expect(html).toContain('id="license-card"');
  expect(html).toContain('aria-label="显示卡密"');
  expect(html).toContain('type="checkbox" checked=""');
});
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `npm test -- --run src/LicenseGate.test.tsx`

Expected: FAIL because `license-crop-radiance` and the crop asset paths do not exist in rendered markup.

- [ ] **Step 3: Commit the red test only if the repository accepts intentionally failing commits**

Do not commit failing code in the normal workflow. Continue immediately to Task 2 so the final commit is green.

### Task 2: Package The Curated Crop Assets

**Files:**
- Create: `frontend/public/crops/license-gate/lilac.png`
- Create: `frontend/public/crops/license-gate/ginseng.png`
- Create: `frontend/public/crops/license-gate/loofah.png`
- Create: `frontend/public/crops/license-gate/orange-jasmine.png`
- Create: `frontend/public/crops/license-gate/ginseng-fruit.png`

- [ ] **Step 1: Copy only the approved mature crop art into Vite's public assets**

Run from the repository root:

```powershell
New-Item -ItemType Directory -Force -Path 'frontend\public\crops\license-gate' | Out-Null
Copy-Item -LiteralPath 'resources\gameConfig\plant_images\stages\作物\丁香花\丁香花_06_成熟.png' -Destination 'frontend\public\crops\license-gate\lilac.png'
Copy-Item -LiteralPath 'resources\gameConfig\plant_images\stages\作物\人参\人参_06_成熟.png' -Destination 'frontend\public\crops\license-gate\ginseng.png'
Copy-Item -LiteralPath 'resources\gameConfig\plant_images\stages\作物\丝瓜\丝瓜_06_结果.png' -Destination 'frontend\public\crops\license-gate\loofah.png'
Copy-Item -LiteralPath 'resources\gameConfig\plant_images\stages\作物\七里香\七里香_06_成熟.png' -Destination 'frontend\public\crops\license-gate\orange-jasmine.png'
Copy-Item -LiteralPath 'resources\gameConfig\plant_images\stages\作物\人参果\人参果_06_成熟.png' -Destination 'frontend\public\crops\license-gate\ginseng-fruit.png'
```

- [ ] **Step 2: Confirm the assets will be exposed with stable public URLs**

Run: `Get-ChildItem 'frontend\public\crops\license-gate' -File | Select-Object Name,Length`

Expected: exactly five named PNG files with non-zero lengths.

### Task 3: Render The Crop Radiance Without Changing License Behavior

**Files:**
- Modify: `frontend/src/LicenseGate.tsx`

- [ ] **Step 1: Add an immutable asset list above the component**

```tsx
const CROP_RADIANCE_IMAGES = [
  '/crops/license-gate/lilac.png',
  '/crops/license-gate/ginseng.png',
  '/crops/license-gate/loofah.png',
  '/crops/license-gate/orange-jasmine.png',
  '/crops/license-gate/ginseng-fruit.png',
  '/crops/license-gate/lilac.png',
  '/crops/license-gate/ginseng.png',
  '/crops/license-gate/loofah.png',
] as const;
```

- [ ] **Step 2: Add the decorative layer as the first child of the existing main element**

```tsx
<div className="license-crop-radiance" aria-hidden="true">
  {CROP_RADIANCE_IMAGES.map((src, index) => (
    <img className="license-crop" src={src} alt="" key={`${src}-${index}`} />
  ))}
</div>
```

Keep the existing header and form as sibling elements after the layer. Do not move the input, button, status, `onSubmit`, `canSubmit`, or state logic into a new component.

- [ ] **Step 3: Run the focused test to verify it passes**

Run: `npm test -- --run src/LicenseGate.test.tsx`

Expected: PASS with all `LicenseGate` tests green.

### Task 4: Implement The Centered Gate And Motion-Safe Styling

**Files:**
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Replace the existing `.license-gate` through its mobile media rules with the approved composition**

Implement these required CSS relationships:

```css
.license-gate { position: relative; display: grid; place-items: center; width: 100vw; height: 100vh; overflow: hidden; isolation: isolate; background: #edf4ee; }
.license-crop-radiance { position: absolute; inset: 50% auto auto 50%; z-index: -1; width: 1px; height: 1px; pointer-events: none; }
.license-gate-form { position: relative; z-index: 1; width: min(426px, calc(100vw - 36px)); padding: 31px 32px 28px; border: 1px solid rgba(214, 255, 103, .35); border-radius: 8px; color: #f8f3e8; background: #1d281f; box-shadow: 0 30px 72px rgba(25, 50, 29, .27), 0 0 0 9px rgba(255, 255, 255, .4); }
.license-card-input input { height: 52px; border: 2px solid #d6ff67; color: #172019; background: #f8ffdd; box-shadow: 0 0 0 4px rgba(214, 255, 103, .12); }
```

Use an eight-item `nth-child` trajectory table for `.license-crop`, with unique viewport-relative destinations, sizes, durations, and negative delays. The `@keyframes license-crop-radiate` animation must alter only `transform` and `opacity`, beginning near the center, moving out, and fading before its repeat. Keep the crop layer below the form and form feedback.

- [ ] **Step 2: Add focus, loading, reduced-motion, and narrow-screen rules**

```css
.license-card-input input:focus { border-color: #d6ff67; box-shadow: 0 0 0 4px rgba(214, 255, 103, .28); }
.license-gate[aria-busy="true"] .license-crop { animation-play-state: paused; }
@media (prefers-reduced-motion: reduce) { .license-crop { animation: none; opacity: .2; transform: translate(calc(-50% + var(--crop-x)), calc(-50% + var(--crop-y))) rotate(var(--crop-turn)); } }
@media (max-width: 640px) { .license-gate-form { width: min(400px, calc(100vw - 32px)); padding: 28px 23px 24px; } .license-gate-bar > span:last-child { display: none; } .license-crop { width: 56px; height: 56px; } }
```

Use `min-width: 0` on the form and controls where needed; preserve the feedback row's minimum height and all current disabled states.

- [ ] **Step 3: Run focused tests and typecheck/build**

Run: `npm test -- --run src/LicenseGate.test.tsx; npm run build`

Expected: all focused tests PASS and Vite emits production assets without TypeScript errors.

### Task 5: Verify The Complete Frontend And Visual States

**Files:**
- Modify: none

- [ ] **Step 1: Run the complete frontend test suite**

Run: `npm test`

Expected: all Vitest test files PASS.

- [ ] **Step 2: Start the local Vite server for visual QA**

Run: `npm run dev -- --host 127.0.0.1 --port 5173`

Expected: Vite reports `http://127.0.0.1:5173/`. If port 5173 is occupied, use the next available port and record it.

- [ ] **Step 3: Inspect the desktop and narrow gate**

At 1280x720 and 390x844, verify the form remains centered, all form text fits, crop art stays behind the form, and the input is the strongest local contrast. Toggle the password visibility control, checkbox, empty-submit disabled state, entered-submit enabled state, loading state, and backend error state.

- [ ] **Step 4: Inspect reduced-motion behavior**

Emulate `prefers-reduced-motion: reduce` in browser devtools. Verify crop icons are static and subdued while no layout, input, or status control changes position.

- [ ] **Step 5: Commit the complete green feature**

```powershell
git add -- frontend/public/crops/license-gate frontend/src/LicenseGate.tsx frontend/src/LicenseGate.test.tsx frontend/src/style.css
git commit -m "feat: add crop radiance license gate"
```

Expected: the feature commit contains only the five public crop assets, gate component/test changes, and gate CSS changes.
