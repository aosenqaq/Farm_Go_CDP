# WMPF 20089 Static Address Compatibility Design

## Goal

Add exact static address support for the installed WeChat WMPF 20089 runtime so Farm_Go can load its Frida hook and complete the CDP debug WebSocket handshake.

## Evidence

Farm_Go selects `resources/wmpf/frida/config/addresses.<version>.json` strictly from the detected WMPF process path. The running 20089 process currently fails because that exact resource does not exist.

The local `20089/extracted/runtime/flue.dll` has SHA-256 `F2FE383D44309C28A8F77101DD71A6DAF7A83835B9DD93B0B09313E0AB52AA2F`. Cross-version PE and instruction analysis identified these 20089 RVAs:

- `AppletIndexContainer::OnLoadStart`: `0x25E0170`
- CDP message filter: `0x2D95AB0`

Both are in the executable `.text` section. Their function-entry instructions, register use, and stack layouts match the verified 20079 hooks after an independently observed `+0x5E0` movement; 20079 values are not reused as fallback values.

## Design

1. Add a focused Frida hook-builder test for version 20089. It proves the exact configuration is selected and carries the verified addresses and established six-step scene path.
2. Add `resources/wmpf/frida/config/addresses.20089.json` containing only the version, two validated RVAs, and the existing WeChat scene path `[64, 1480, 8, 1416, 16, 456]`.
3. Keep exact-version lookup unchanged. No nearest-version fallback, dynamic scanner, or farm-operation change is included.
4. Run the focused and package-level WMPF tests, then build Farm_Go. The user can reopen the mini-program to prove the live Frida load and CDP ready transition.

## Acceptance Criteria

- The application no longer reports a missing `addresses.20089.json` resource.
- The generated hook script contains the exact 20089 configuration.
- WMPF tests and the Go build pass.
- Live validation reaches Frida loaded and WeChat CDP ready without running farm mutations.
