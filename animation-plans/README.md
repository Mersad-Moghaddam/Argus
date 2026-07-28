# Argus Motion Audit and Implementation Plans

Audit baseline: commit `b576998`
Date: 2026-07-28
Scope: `frontend/styles.css`, `frontend/app.js`, `frontend/projects.js`
Constraint: planning only; no source animation code was changed.

## Executive summary

Argus has a coherent motion vocabulary and good timing tokens, but motion is applied too uniformly. The strongest pieces are fast **Press/Tap feedback**, interruptible toast transitions and a restrained **Scale in** modal. The main problems are perpetual decorative **Loop**/**Pulse**, **Fade in** on every high-frequency tab switch, reduced-motion behavior that removes all useful feedback, and hover behavior that is not limited to precise pointing devices.

The recommended direction is operational: motion should communicate request state, hierarchy or causality. Navigation itself should be immediate.

## Findings

| Priority | Finding | Evidence | Recommendation |
|---|---|---|---|
| P1 | Brand **Loop** runs forever without state meaning | `frontend/styles.css:194-216` | Remove loop; keep a static ring or one-time reveal |
| P1 | Every tab uses **Fade in** plus vertical translation | `frontend/styles.css:564-583` | Make tab changes instant, especially keyboard navigation |
| P1 | reduced motion forces all animation/transition to `0.001ms` | `frontend/styles.css:145-154` | Remove spatial/loop motion selectively; preserve color/opacity feedback |
| P1 | Refresh **Pulse** runs continuously, not only during a refresh | `frontend/styles.css:1132-1163` | Tie Pulse to active request and use a static final state |
| P2 | All hover styles can apply on touch | examples `frontend/styles.css:300-343,704-744` | Gate decorative hover with `hover:hover` and `pointer:fine` |
| P2 | Toast motion is sound but close target is tiny | `frontend/styles.css:965-1028` | Preserve interruptible transition; increase target and use logical direction |
| P2 | **Shimmer** loops for every skeleton | `frontend/styles.css:929-949` | Disable movement for reduced motion and avoid for sub-200ms loads |
| P3 | Modal entrance is appropriate but dialog controller is incomplete | `frontend/styles.css:1035-1074`; `frontend/projects.js:1600-1624` | Keep restrained Scale in after focus/inert behavior is corrected |

## Current motion inventory

| Exact term | Trigger | Frequency | Duration | Current quality |
|---|---|---:|---:|---|
| **Loop** (`sweep-ring`) | always | continuous | 3.4s cycle | distracting |
| **Loading spinner** | submit/loading | occasional | 0.7s cycle | appropriate |
| **Fade in** + translate | every tab | high | 280ms | excessive |
| **Shimmer** | skeleton | repeated | 1.4s cycle | useful with limits |
| **Slide in** + **Scale in** | toast | occasional | 280ms | good and interruptible |
| **Scale in** | modal open | occasional | 280ms | appropriate |
| **Pulse** | refresh indicator | continuous | 2s cycle | lacks state meaning |
| **Press/Tap feedback** | button active | high | 120ms | good |

## Before / After / Why

| Before | After | Why |
|---|---|---|
| Perpetual scanning ring | static brand mark; optional one-time first-load reveal | decorative movement should not compete with operational alerts |
| Animated tab panels | immediate panel swap | high-frequency and keyboard interactions need low latency |
| Global animation kill | semantic reduced-motion overrides | users still need state feedback |
| Always-pulsing green dot | Pulse only while request is in flight | animation gains a clear cause |
| Ungated hover | fine-pointer hover only | prevents sticky touch feedback |

## Plan order

1. [Make tab navigation immediate](02-instant-tab-navigation.md)
2. [Implement semantic reduced motion](03-semantic-reduced-motion.md)
3. [Make refresh motion state-driven](04-state-driven-refresh-indicator.md)
4. [Remove the perpetual brand loop](01-remove-perpetual-brand-loop.md)
5. [Harden touch hover and toast feedback](05-touch-hover-and-toast-polish.md)

Items 1–3 have the highest user impact. Each document is standalone and can be assigned independently, but reduced-motion verification should run again after all motion changes land.

## Shared verification matrix

- Chrome, Firefox and Safari current stable.
- Keyboard tab navigation with arrow keys: no panel animation and no delayed focus.
- `prefers-reduced-motion: reduce`: no translation, scale, shimmer, loop or pulse.
- Color/opacity/focus feedback remains visible with reduced motion.
- Touch simulation: no sticky hover.
- Slow network: skeleton appears only after a short loading-delay threshold if implemented.
- Rapid toast add/remove: transition remains interruptible and no queued animation.
- No layout shift from changing animation states.
