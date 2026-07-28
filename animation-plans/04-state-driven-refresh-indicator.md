# Plan 04 — Make the refresh Pulse state-driven

Baseline: `b576998`
Owner profile: frontend engineering
Estimated effort: medium

## Objective

Use **Pulse** only while a refresh request is in flight. At rest, the dot should be static and its text should report freshness.

## Evidence

`frontend/styles.css:1132-1163` always applies:

```css
.refresh-dot {
  ...
  animation: pulse 2s infinite;
}
```

`frontend/app.js:418-446` owns the refresh lifecycle, and `frontend/app.js:624-631` owns the countdown.

## State contract

```text
idle       static dot, "Updated 12s ago"
refreshing Pulse, "Refreshing…", aria-busy=true
success    static ok dot, timestamp
error      static down dot, concise error and Retry action
offline    static muted dot, "Offline"
```

## CSS implementation

```css
.refresh-dot {
  animation: none;
}

@media (prefers-reduced-motion: no-preference) {
  .refresh-indicator.is-refreshing .refresh-dot {
    animation: pulse 1.2s var(--ease-out) infinite;
  }
}
```

## JavaScript implementation outline

At the single refresh coordinator:

```js
setRefreshState('refreshing');
try {
  await loadDashboardData();
  setRefreshState('success', { updatedAt: new Date() });
} catch (error) {
  setRefreshState('error', { error });
}
```

Do not create a new timer per view. Consolidate overlapping legacy/project polling as planned in the UX/architecture documents.

## Accessibility

- container uses `aria-busy="true"` only during an actual refresh;
- a small status region announces failure and manual success, but silent background success should not announce every 20–30 seconds;
- color is accompanied by text;
- offline is derived from network/request state, not only `navigator.onLine`.

## Acceptance criteria

- no Pulse at rest;
- Pulse starts and stops with the real promise lifecycle;
- overlapping refresh calls are deduplicated or cancelled;
- hidden tabs do not animate or poll;
- background success does not spam assistive technology;
- reduced motion never pulses.

## Verification

1. Throttle network to observe the refreshing state.
2. Force 500 and offline responses.
3. Switch tabs/background the page.
4. Trigger manual refresh during an active refresh; ensure one coherent state.
