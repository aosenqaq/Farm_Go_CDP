# License Gate Crop Radiance Design

## Goal

Replace the sparse card-key validation page with a centered authorization surface that has a decisive input focus. Mature crop images already extracted under `resources/gameConfig/plant_images` radiate slowly from the center behind the form, giving the gate a Farm Go-specific identity without competing with validation work.

## Scope

- Update only the `LicenseGate` visual composition and its associated global CSS.
- Preserve card masking, visibility toggle, remembered-card behavior, validation submission, loading state, and existing Chinese copy.
- Do not introduce new backend data, network requests, or license behavior.

## Composition

`LicenseGate` keeps the existing top-level form and accessible input semantics, but adds a decorative background layer before the form. The form becomes a dark ink-green card centered in the full viewport. The existing header remains as compact brand context rather than a full-width dark bar.

The form card uses a narrow lime edge accent, a dark surface, and a pale-lime card input. The card input has a 2px lime border and a restrained focus ring. This yields explicit contrast at three levels: quiet pale-green page, dark authorization container, pale-lime interactive field.

The action button remains lime, with the existing `ShieldCheck` icon and submit states. The visibility button stays within the input's trailing area and keeps its existing accessible name.

## Crop Radiance Layer

Use a fixed, curated group of mature crop PNGs from the existing extracted crop imagery. At runtime the React component renders the images as decorative, `aria-hidden` elements. No image is loaded from the network and crop data is not fetched.

Each image starts near the visual center of the page, then moves toward an assigned viewport-relative destination while rotating slightly, reducing opacity, and finally restarting. Staggered negative delays ensure the screen appears alive on load rather than emitting a synchronized burst. The layer is behind the gate, cannot receive pointer events, and uses low opacity plus a muted drop shadow.

The animation uses `transform` and `opacity` only. It must not affect document layout or obscure input text, labels, controls, loading feedback, or error text. `prefers-reduced-motion: reduce` disables the animation and leaves a small, static, subdued crop field around the card.

## Asset Delivery

The required mature crop icons are copied from the existing extracted resource tree into a frontend public asset folder during implementation. This gives Vite/Wails stable packaged paths and avoids coupling the running frontend to the source-only `resources` filesystem path. Only a small curated set is copied, not the full crop catalog.

## Responsive Behavior

The authorization card width is constrained by the viewport with comfortable mobile side gutters. On narrow screens, the secondary header label is hidden, crop images are reduced in size and density, and the form remains vertically centered. The page continues to support the current minimum desktop dimensions.

## Error And Loading States

Existing error and loading text remains inside the form flow. The feedback line reserves height so status changes do not move the submit button. The form's `aria-busy`, disabled input, disabled remember checkbox, and disabled submit semantics remain unchanged.

## Verification

- Extend the focused `LicenseGate` tests to assert the new decorative crop layer is hidden from assistive technology and that the existing control semantics remain intact.
- Run the focused license gate test, full frontend test suite, and frontend production build.
- Start the Vite development server and visually inspect the gate at desktop and narrow viewport widths, including an emulated reduced-motion mode.
