# Plan 02 — Make high-frequency tab navigation immediate

Baseline: `b576998`
Owner profile: frontend engineering
Estimated effort: small

## Objective

Remove **Fade in** and vertical translation from every tab change. Tab selection is a high-frequency navigation action, including arrow-key navigation, and should respond immediately.

## Evidence

`frontend/styles.css:564-583`:

```css
.tab-panel {
  display: none;
}

.tab-panel.active {
  display: block;
  animation: fade-in var(--dur-slow) var(--ease-out);
}

@keyframes fade-in {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
```

## Implementation

```css
.tab-panel {
  display: none;
}

.tab-panel.active {
  display: block;
}
```

Remove `@keyframes fade-in` if no other selector references it.

Do not replace it with Web Animations API, a timeout or a shorter translation. The goal is direct manipulation, not a different animation technology.

If a future full-page route transition needs continuity, implement it at the app-shell content boundary and never on roving-tab keyboard selection.

## Acceptance criteria

- mouse click and Left/Right arrow navigation swap panels in the same frame;
- focus remains on the selected tab according to the existing roving-tab pattern;
- no opacity flash or translated layout;
- screen reader announcements are not delayed;
- no unused `fade-in` keyframes remain.

## Verification

1. Hold the Right Arrow key across tabs; content tracks selection without queued motion.
2. Test at 6× CPU throttling; selected tab and panel never visibly disagree.
3. Test reduced motion; behavior is the same.
4. Confirm modal and toast motion are unaffected.
