# Friend List Level Sort Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add default, ascending-level, and descending-level ordering to the social friend list through a compact select control.

**Architecture:** Keep ordering in `SocialView`. An exported helper creates a stable sorted copy after the existing filter/search pass; React component state retains the choice only for the live application session.

**Tech Stack:** React 18, TypeScript, Vitest, react-test-renderer, CSS.

---

### Task 1: Add Deterministic Friend Ordering

**Files:**
- Modify: `frontend/src/views/SocialView.tsx:56-65, 485-589`
- Test: `frontend/src/views/SocialView.test.tsx:1-125`

- [ ] **Step 1: Write the failing helper test**

Import `sortFriendsByLevel`, then add this test before modifying production code:

```tsx
it('keeps default friend order and stably sorts numeric levels with unknown levels last', () => {
  const friends = [
    { ...state.friends[0], gid: 1, displayName: '等级 12', level: 12 },
    { ...state.friends[0], gid: 2, displayName: '未知等级', level: undefined },
    { ...state.friends[0], gid: 3, displayName: '等级 3-A', level: 3 },
    { ...state.friends[0], gid: 4, displayName: '等级 3-B', level: 3 },
  ];

  expect(sortFriendsByLevel(friends, 'default').map((friend) => friend.gid)).toEqual([1, 2, 3, 4]);
  expect(sortFriendsByLevel(friends, 'level_asc').map((friend) => friend.gid)).toEqual([3, 4, 1, 2]);
  expect(sortFriendsByLevel(friends, 'level_desc').map((friend) => friend.gid)).toEqual([1, 3, 4, 2]);
});
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `npm test -- --run src/views/SocialView.test.tsx`

Expected: Vitest reports that `sortFriendsByLevel` is not exported.

- [ ] **Step 3: Implement the minimal sort type and helper**

Add immediately below `FriendRow`:

```tsx
export type FriendSortMode = 'default' | 'level_asc' | 'level_desc';

export function sortFriendsByLevel(friends: FriendRow[], sortMode: FriendSortMode): FriendRow[] {
  if (sortMode === 'default') return friends;
  const direction = sortMode === 'level_asc' ? 1 : -1;
  return friends
    .map((friend, index) => ({ friend, index, level: Number.isFinite(friend.level) ? Number(friend.level) : null }))
    .sort((left, right) => {
      if (left.level === null && right.level === null) return left.index - right.index;
      if (left.level === null) return 1;
      if (right.level === null) return -1;
      return (left.level - right.level) * direction || left.index - right.index;
    })
    .map(({ friend }) => friend);
}
```

- [ ] **Step 4: Apply the helper after filtering**

Add this state after `filter`:

```tsx
const [friendSortMode, setFriendSortMode] = useState<FriendSortMode>('default');
```

Wrap the result of the existing `current.friends.filter(...)` call in `sortFriendsByLevel(..., friendSortMode)`, and add `friendSortMode` to the memo dependencies. Do not mutate `current.friends`.

- [ ] **Step 5: Run the focused test to verify it passes**

Run: `npm test -- --run src/views/SocialView.test.tsx`

Expected: the new test and the existing SocialView tests pass.

### Task 2: Render and Exercise the Sort Select

**Files:**
- Modify: `frontend/src/views/SocialView.tsx:757-779`
- Modify: `frontend/src/style.css:1870-1900`
- Test: `frontend/src/views/SocialView.test.tsx:307-317`

- [ ] **Step 1: Write the failing interaction test**

Add this test before rendering the control:

```tsx
it('sorts the friend table from the compact level selector', () => {
  const sortedState: SocialState = {
    ...state,
    summary: { ...state.summary, totalFriends: 3 },
    friends: [
      { ...state.friends[0], gid: 1, displayName: '等级 12', level: 12, stealable: true },
      { ...state.friends[0], gid: 2, displayName: '等级 3', level: 3, stealable: true },
      { ...state.friends[1], gid: 3, displayName: '非可偷', level: 99, stealable: false },
    ],
  };
  const renderer = TestRenderer.create(<SocialView state={sortedState} onRefresh={() => undefined} onAction={() => undefined} />);
  const select = renderer.root.findByProps({ 'aria-label': '好友排序' });

  expect(JSON.stringify(renderer.toJSON())).toContain('等级升序');
  expect(JSON.stringify(renderer.toJSON())).toContain('等级倒序');
  act(() => select.props.onChange({ target: { value: 'level_asc' } }));
  act(() => findButton(renderer.root, '可偷').props.onClick());

  const rows = renderer.root.findAllByProps({ className: 'social-friend-name' });
  expect(rows.map((row) => textFromChildren(row.props.children))).toEqual(['等级 3', '等级 12']);
});
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `npm test -- --run src/views/SocialView.test.tsx`

Expected: react-test-renderer cannot find `aria-label="好友排序"`.

- [ ] **Step 3: Render the selected A control**

Place this directly after `.social-search` and before the filter-chip map:

```tsx
<select
  aria-label="好友排序"
  className="social-sort-select"
  value={friendSortMode}
  onChange={(event) => setFriendSortMode(event.target.value as FriendSortMode)}
>
  <option value="default">默认</option>
  <option value="level_asc">等级升序</option>
  <option value="level_desc">等级倒序</option>
</select>
```

- [ ] **Step 4: Style the control with the existing toolbar language**

Add this CSS rule beside `.social-search`:

```css
.social-sort-select {
  min-height: 34px;
  padding: 0 28px 0 10px;
  border: 1px solid #e2d4b8;
  border-radius: 7px;
  background: #fffaf0;
  color: #4f493d;
  font: inherit;
  font-size: 12px;
  font-weight: 800;
}
```

- [ ] **Step 5: Run the focused test to verify it passes**

Run: `npm test -- --run src/views/SocialView.test.tsx`

Expected: selector interaction and all existing SocialView tests pass.

### Task 3: Verify the Finished Frontend Slice

**Files:**
- Modify: `frontend/src/views/SocialView.tsx`
- Modify: `frontend/src/views/SocialView.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Run the complete frontend test suite**

Run: `npm test`

Expected: exit code 0 with all frontend tests passing.

- [ ] **Step 2: Run the frontend production build**

Run: `npm run build`

Expected: TypeScript checking and Vite build finish with exit code 0.

- [ ] **Step 3: Review scope and diff**

Run: `git diff --check; git diff -- frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx frontend/src/style.css`

Expected: no whitespace errors; the diff only contains the sort helper, state/control, focused styles, and regression tests.

- [ ] **Step 4: Commit the feature**

```bash
git add frontend/src/views/SocialView.tsx frontend/src/views/SocialView.test.tsx frontend/src/style.css
git commit -m "feat: add friend level sorting"
```
