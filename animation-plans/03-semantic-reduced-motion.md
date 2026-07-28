# Plan 03 — Replace the global animation kill with semantic reduced motion

Baseline: `b576998`
Owner profile: frontend/accessibility engineering
Estimated effort: medium

## Objective

Respect `prefers-reduced-motion` by removing spatial movement and continuous motion while preserving meaningful focus, color and immediate opacity feedback.

## Evidence

`frontend/styles.css:145-154`:

```css
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.001ms !important;
    scroll-behavior: auto !important;
  }
}
```

This catches all motion but also removes useful non-spatial feedback and relies on near-zero animation events.

## Implementation

Replace the universal duration override with targeted rules:

```css
@media (prefers-reduced-motion: reduce) {
  html {
    scroll-behavior: auto;
  }

  .brand-mark::after,
  .skeleton,
  .refresh-dot,
  .btn-spinner {
    animation: none;
  }

  .modal,
  .toast,
  .toast[data-entering="true"],
  .toast.leaving {
    transform: none;
  }

  .modal {
    animation: none;
  }

  .toast {
    transition: opacity 80ms linear;
  }

  button {
    transition-property: color, background-color, border-color, filter;
  }

  button:active {
    transform: none;
  }
}
```

Implementation must be adjusted to the final selectors after Plans 01/02/04. A static fallback for `.btn-spinner` needs visible text such as “Working…”; never remove the only loading indication.

## Acceptance criteria

- no translate, scale, pulse, shimmer, sweep or spinner rotation in reduced mode;
- loading remains exposed through text and disabled/busy state;
- focus ring, color changes and status text remain immediate;
- toast appear/disappear may use a very short opacity-only transition;
- no code waits for an `animationend` event that reduced mode prevents.

## Verification

1. Enable OS reduced motion before page load.
2. Exercise loading, tab, toast, modal, refresh and button press states.
3. Search JavaScript for `animationend`/`transitionend`; verify no lifecycle depends on the removed motion.
4. Run keyboard and screen-reader flows to confirm state feedback remains.
