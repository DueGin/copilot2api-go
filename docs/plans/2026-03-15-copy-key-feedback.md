# Copy Key Feedback Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make account keys and pool keys copy on click, turn green after successful copy, and only show success feedback when the clipboard write actually succeeds.

**Architecture:** Introduce a shared copy-feedback primitive in the web UI so both key surfaces use the same success state, timing, and failure behavior. Extend the clipboard helper to return a success result, then cover the shared behavior with focused component tests before wiring it into existing screens.

**Tech Stack:** React 19, TypeScript, Vite, Vitest, Testing Library

---

### Task 1: Add frontend test infrastructure

**Files:**
- Modify: `web/package.json`
- Create: `web/vitest.config.ts`
- Create: `web/src/test/setup.ts`
- Create: `web/src/test/renderWithLocale.tsx`

**Step 1: Write the failing test**

Create a component test file that imports Vitest globals and Testing Library helpers; the test run should fail because the project has no test command or Vitest config yet.

**Step 2: Run test to verify it fails**

Run: `npm test -- --run`
Expected: FAIL because no `test` script exists.

**Step 3: Write minimal implementation**

Add a `test` script and minimal Vitest + jsdom setup for React component tests.

**Step 4: Run test to verify it passes**

Run: `npm test -- --run`
Expected: Vitest starts and reports test results instead of missing-script/config errors.

**Step 5: Commit**

```bash
git add web/package.json web/vitest.config.ts web/src/test/setup.ts web/src/test/renderWithLocale.tsx
git commit -m "test: add frontend component test setup"
```

### Task 2: Add failing tests for copy feedback

**Files:**
- Create: `web/src/components/CopyableSecret.test.tsx`
- Test: `web/src/components/CopyableSecret.test.tsx`

**Step 1: Write the failing test**

Add tests covering:
- successful copy switches the label to `Copied!` and turns the secret green
- failed copy keeps the original label and color

**Step 2: Run test to verify it fails**

Run: `npm test -- --run CopyableSecret`
Expected: FAIL because the shared component does not exist yet.

**Step 3: Write minimal implementation**

Create the minimal shared component API expected by the tests, without wiring it into the app yet.

**Step 4: Run test to verify it passes**

Run: `npm test -- --run CopyableSecret`
Expected: PASS for the new component tests.

**Step 5: Commit**

```bash
git add web/src/components/CopyableSecret.test.tsx web/src/components/CopyableSecret.tsx
git commit -m "test: cover copy feedback behavior"
```

### Task 3: Implement shared clipboard success handling

**Files:**
- Modify: `web/src/clipboard.ts`
- Modify: `web/src/components/CopyableSecret.tsx`

**Step 1: Write the failing test**

Extend the component tests so they assert success feedback only appears after a resolved clipboard write and not after a rejected one.

**Step 2: Run test to verify it fails**

Run: `npm test -- --run CopyableSecret`
Expected: FAIL because the clipboard helper currently does not expose success/failure.

**Step 3: Write minimal implementation**

Make `copyText` async and return `true`/`false`, including fallback behavior.

**Step 4: Run test to verify it passes**

Run: `npm test -- --run CopyableSecret`
Expected: PASS.

**Step 5: Commit**

```bash
git add web/src/clipboard.ts web/src/components/CopyableSecret.tsx web/src/components/CopyableSecret.test.tsx
git commit -m "feat: return clipboard copy result"
```

### Task 4: Wire the shared UI into account and pool keys

**Files:**
- Modify: `web/src/components/AccountCard.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/i18n.tsx`

**Step 1: Write the failing test**

Add or extend tests so the shared component supports the exact labels used by account keys and pool keys.

**Step 2: Run test to verify it fails**

Run: `npm test -- --run CopyableSecret`
Expected: FAIL until the existing screens adopt the shared component API.

**Step 3: Write minimal implementation**

Replace duplicated inline copy logic with the shared component in both places while preserving show/hide and regenerate actions.

**Step 4: Run test to verify it passes**

Run: `npm test -- --run CopyableSecret`
Expected: PASS.

**Step 5: Commit**

```bash
git add web/src/components/AccountCard.tsx web/src/App.tsx web/src/i18n.tsx web/src/components/CopyableSecret.tsx
git commit -m "feat: unify key copy feedback"
```

### Task 5: Verify the final behavior

**Files:**
- Modify: `web/package.json`
- Test: `web/src/components/CopyableSecret.test.tsx`

**Step 1: Write the failing test**

No new test code; use the full verification suite as the final gate.

**Step 2: Run test to verify it fails**

Not applicable for this task.

**Step 3: Write minimal implementation**

No implementation changes unless verification exposes issues.

**Step 4: Run test to verify it passes**

Run:
- `npm test -- --run`
- `npm run build`

Expected: both commands PASS.

**Step 5: Commit**

```bash
git add web/package.json web/src web/vitest.config.ts docs/plans
git commit -m "feat: improve key copy feedback"
```
