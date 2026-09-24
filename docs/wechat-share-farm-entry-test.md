# WeChat Shared Farm Entry Test

Status: verified on 2026-07-28

## Scope

This record covers one user-authorized WeChat mini-program farm entry through
the existing game-side share route. It documents the UI transition only; it
does not authorize farm actions, social actions, or persistent automation.

## Verified Route

The active runtime was `wechat_cdp`, connected, and ready. One call was made:

```text
gameCtl.enterFarmByGid(1189167721, { reason: 5, waitMs: 5000 })
```

`gameCtl.enterFarmByGid` resolved the game-native
`System:chunks:///_virtual/FarmUtil.ts` implementation and called
`FarmUtil.enterFarm`. The client constructed the share-entry request rather
than replaying a captured packet.

## Evidence

| Check | Observed result |
| --- | --- |
| Runtime | `wechat_cdp`, connected, ready |
| Target | `gid=1189167721` |
| Client state after entry | Friend farm; `isInVisit=true`, `isOwerFarm=false` |
| Current farm owner | `gid=1189167721` |
| Scene events | `FARM_LEAVE`, `FARM_READY`, `FARM_ENTER`, `FARM_MAP_INITIALIZED` |
| Wire request | One distinct `gamepb.visitpb.VisitService.Enter` request |
| Decoded fields | Field 1 target GID, field 2 reason `5`, field 3 zero, and opaque length-delimited field 7 |

The runtime spy recorded two `Enter` entries because it instruments both the
network wrapper and channel wrapper. They were the same wire message, not two
game requests.

## Contract

The share-entry route is currently verified across the QQ and WeChat runtimes
when the game client invokes `FarmUtil.enterFarm(targetGid, 5)`. For a
required in-game scene transition, use `gameCtl.enterFarmByGid` with numeric
reason `5` and verify both ownership and `FARM_ENTER`.

A hand-built `VisitService.Enter` request that contains only target GID and
reason can return visit data, but it is not proof of a frontend scene change.
It must not replace the game-side route for UI-entry claims.

## Friend-Application Share Context

The in-game "add game friend" control is a separate, share-authorized flow:

- UI handler: `UIStangerAddFriend::UIFriendApply.onClick()` on
  `btn_addFriend`.
- Request: `gamepb.friendpb.FriendService.ApplyFriend`.
- Verified client path: `UIFriendApply.onClick` passes the current share
  context as the third argument to `FriendManager.reqApplyFriend`; that method
  writes the argument directly to protobuf field 2, defaulting to an empty
  string when no context is present.
- Field 1 is the target GID.
- When the current share context authorizes a friend application, field 2 is
  a 75-byte ASCII value shaped as `<64 lowercase hex characters>:<targetGid>`.
  The prefix is opaque, sensitive session material and is not recorded here.
- When the share context does not authorize the application, the same request
  carries an empty field 2. This occurred even though the game had entered the
  farm through a normal share scene with a `shareTicket` and a 32-byte
  `VisitService.Enter` field 7.

Field 2 is therefore neither derivable from GID nor interchangeable with the
share ticket or `Enter` field 7. It is observable from the client during an
authorized manual application, either at the third `reqApplyFriend` argument
or as the outgoing protobuf field 2, but must be treated as current
share-session authorization material. A later fresh authorized context
produced a populated field 2 for the same target whose earlier request had an
empty field 2, which also rules out using target identity or share-scene
presence as the condition.

The capture listener did not expose a corresponding `ApplyFriend` callback,
so a populated field 2 proves the client supplied the authorization context;
it does not by itself prove the server accepted the application. Do not
persist, replay, synthesize, or log the opaque prefix.

## Boundaries

- Send one entry request per test window; do not retry on failure.
- Reset runtime spies before the request and inspect them after the transition.
- Do not persist or replay session tokens, opaque field values, or raw packets.
- Do not automate or replay friend-application requests. The observations
  above document only the client-side request contract.
- This result does not validate crop collection, help, mischief, or any other
  action in the target farm.
