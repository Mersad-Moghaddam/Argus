# Plan 01 — Remove the perpetual brand Loop

Baseline: `b576998`
Owner profile: frontend/design engineering
Estimated effort: small

## Objective

Stop the decorative `sweep-ring` **Loop** from running continuously. The brand should not resemble an active alert or compete with incident status.

## Evidence

`frontend/styles.css:194-216`:

```css
.brand-mark::after {
  content: "";
  position: absolute;
  inset: -5px;
  border-radius: 14px;
  border: 1px solid var(--signal);
  opacity: 0;
  animation: sweep-ring 3.4s var(--ease-out) infinite;
}

@keyframes sweep-ring {
  0% {
    opacity: 0.55;
    transform: scale(0.86);
  }
  70%,
  100% {
    opacity: 0;
    transform: scale(1.22);
  }
}
```

## Implementation

Preferred smallest change:

```css
.brand-mark::after {
  content: "";
  position: absolute;
  inset: -4px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--signal) 42%, transparent);
  opacity: 1;
}
```

Then remove the unused `@keyframes sweep-ring`.

Optional one-time reveal is allowed only if product testing finds the static mark too flat:

```css
@media (prefers-reduced-motion: no-preference) {
  .app-ready .brand-mark::after {
    animation: brand-reveal 280ms var(--ease-out) both;
  }
}
```

Do not run it on route changes or repeat visits. A static solution is preferred.

## Acceptance criteria

- no infinite animation on the brand;
- visual identity remains recognizable in dark and light themes;
- ring never looks like an incident severity indicator;
- reduced-motion mode is identical or calmer;
- no new JavaScript timer or persisted flag is added merely for decoration.

## Verification

1. Open the dashboard and watch the brand for 10 seconds; nothing repeats.
2. Toggle themes; ring contrast remains subtle.
3. Record a performance trace; no continuous animation work comes from the brand pseudo-element.
