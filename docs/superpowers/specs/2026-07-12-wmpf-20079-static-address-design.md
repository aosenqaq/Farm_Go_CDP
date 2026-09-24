# WMPF 20079 Static Address Compatibility Design

## Goal

Add exact static address support for the currently installed WeChat WMPF 20079 runtime so Farm_Go can load its Frida hook and complete the CDP debug WebSocket handshake.

Business automation behavior is outside this change. Planting, warehouse, friend, reward, and other `gameCtl` operations will not be exercised.

## Current Failure

The running `WeChatAppEx.exe` is loaded from the `RadiumWMPF/20079` plugin directory. Farm_Go selects a Frida address file by this exact version, but the repository currently ends at `addresses.20005.json`. Consequently, 20079 cannot build its hook script from the embedded resources.

Reusing 20005 offsets is unsafe because both hook addresses are relative to `flue.dll` and may move between builds.

## Design

1. Locate the 20079 `AppletIndexContainer::OnLoadStart` and CDP filter function entry points in the installed `flue.dll` using instruction and reference evidence from the known hook behavior.
2. Validate both relative virtual addresses against the local 20079 PE image:
   - each address is inside an executable section;
   - each address is aligned to a decoded instruction boundary;
   - the surrounding instructions match the expected function role rather than only a nearby byte pattern.
3. Add `resources/wmpf/frida/config/addresses.20079.json` with the verified offsets and the unchanged six-step scene path used by 20005 unless runtime evidence proves the object layout changed.
4. Add a focused test that proves embedded/local resource validation and hook-script construction support version 20079. The test must fail before the config is added and pass afterward.

No fallback to the nearest version and no dynamic signature scanner will be added.

## Online Verification

After targeted tests and build checks pass:

1. Start Farm_Go against the currently running WMPF 20079 process.
2. Confirm the Frida helper reports `FRIDA_LOADED` without an attach, script, or access-violation error.
3. Trigger or reopen the mini-program through the normal connection flow.
4. Confirm the debug WebSocket handshake completes and the runtime target reaches its connected/ready state.

The verification stops at connection readiness. It will not call mutating or business-level `gameCtl` methods.

## Failure Handling

If either hook crashes the WMPF process or the expected callback is not observed, stop the live attempt and return to binary analysis. Do not layer guesses or reuse an older version's offset.

## Acceptance Criteria

- Version 20079 has an exact address configuration in the embedded resource tree.
- The focused 20079 configuration test passes.
- Existing WMPF Frida and link tests remain green.
- The live 20079 process loads the hook and completes the debug WebSocket handshake.
- No farm automation feature or protocol behavior is changed.
