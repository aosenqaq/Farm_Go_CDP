# QQ GID-to-OpenID Add-Friend Test Design

## Goal

Perform one user-authorized QQ add-friend experiment for a controlled test
account. Resolve the target OpenID from the current QQ Farm visit reply, then
invoke the QQ native add-friend API once with the user-supplied verification
message.

## Scope

Add one diagnostic runtime operation in `resources/wmpf/button.js`.

1. Query `gamepb.visitpb.VisitService.Enter` with the supplied target GID and
   reason `5`.
2. Read only `rawReply.basic.gid` and `rawReply.basic.open_id` from that
   response.
3. Continue only when the response GID exactly matches the requested GID and
   `open_id` is a non-empty string.
4. Locate `qq.addFriendByOpenId` and invoke it exactly once with that OpenID
   and the supplied verification message.
5. Return a bounded result and rely on the existing native-friend Spy for the
   call and callback evidence.

## Safety Boundaries

- Do not use `binded_communities[].openid`; it is a different response field.
- Do not accept a caller-supplied OpenID, infer an OpenID from another field,
  or use a stale captured value.
- Do not retry a failed query, failed validation, unavailable native API, or
  native callback failure.
- Do not run batch operations or add generic friend-management behavior.
- On any failed precondition, return a failure result without invoking QQ.

## Data Flow

`targetGid` is encoded into the proven visit request. The live reply is decoded
with the existing visit codec. The operation validates `basic.gid`, extracts
`basic.open_id`, calls the native API with the original QQ receiver, and
returns whether the native call was invoked. Existing native Spy events record
the native invocation and callback result.

## Tests

Add VM regression coverage that proves the operation:

- invokes `VisitService.Enter` before the native API;
- rejects a mismatched reply GID and missing `basic.open_id` without invoking
  QQ;
- uses `basic.open_id`, rather than the bound-community OpenID;
- invokes `addFriendByOpenId` only once with the supplied verification message;
- preserves the native API receiver and callback behavior.

## Acceptance Criteria

1. A clean live capture contains one `VisitService.Enter` query and one
   `qq.addFriendByOpenId` native event.
2. The queried reply GID equals the requested GID.
3. The native event OpenID equals the validated `basic.open_id` from that same
   live query.
4. The capture contains no UI click events, guessed requests, or retries.
