# Plan 05 — Gate hover and preserve interruptible toast feedback

Baseline: `b576998`
Owner profile: frontend/design engineering
Estimated effort: medium

## Objective

Avoid sticky hover states on touch, keep toast **Slide in**/**Scale in** interruptible, and make its close control accessible.

## Evidence

- global hover selectors such as `button:hover`: `frontend/styles.css:300-343`;
- table hover and sortable headers: `frontend/styles.css:704-744`;
- toast transition and 2px close padding: `frontend/styles.css:965-1028`;
- toast removal is class-based and therefore interruptible: `frontend/app.js:52-67`.

## Hover implementation

Move visual-only hover declarations under:

```css
@media (hover: hover) and (pointer: fine) {
  button:hover {
    filter: brightness(1.08);
  }

  tbody tr:hover {
    background: var(--ink-raised-2);
  }

  /* Move the remaining existing :hover declarations here. */
}
```

Do not hide required information behind hover. Focus-visible styles remain outside the media query.

## Toast target and logical motion

```css
.toast-close {
  min-width: 28px;
  min-height: 28px;
  padding: 4px;
  display: inline-grid;
  place-items: center;
}
```

Prefer logical inline direction for localization. If the UI becomes RTL, the entry direction should follow the notification edge or be reduced to opacity rather than hard-coded rightward motion.

Keep transitions, not keyframe animations:

```css
.toast {
  transition:
    opacity var(--dur-slow) var(--ease-out),
    transform var(--dur-slow) var(--ease-out);
}
```

This allows reversal/removal without queued animation.

## Behavior changes

- pause auto-dismiss while pointer is over the toast or keyboard focus is inside it;
- resume with remaining duration, not a fresh full timeout if practical;
- errors requiring action do not auto-dismiss;
- close button has an accessible name that includes context if multiple toasts exist;
- focus is not moved to routine toasts.

## Acceptance criteria

- touch emulation has no sticky hover;
- every hover affordance has an equivalent focus-visible affordance;
- toast close target is at least 24×24 CSS px, preferably 28 or larger;
- rapid toast creation/removal produces no snapping or queued animation;
- error content stays long enough to act on;
- reduced-motion uses opacity only as defined in Plan 03.

## Verification

1. Real touch device or Chrome touch emulation.
2. Keyboard focus through all toast actions.
3. Create five toasts quickly; dismiss middle/first/last.
4. Toggle RTL in dev tools if localization is planned.
5. Verify no layout shift in the fixed toast stack.
