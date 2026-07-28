# Skills and Plugins Decision

Date: 2026-07-28

## Outcome

No duplicate third-party skill was installed. The best capabilities for this
repository are already active, and adding overlapping marketplace packages
would increase instruction and supply-chain surface without improving the
delivery plan.

The installed Playwright skill was verified by running its wrapper against the
local static UI. That pass found the missing hidden-state contract documented
as a P0 issue. The browser package was fetched only for the session, and
temporary artifacts were removed.

Plugin installation could not be completed in this session: the available
Plugin Management surface exposed inspection/removal/permission functions but
did not expose the required `search_plugins` and `suggest_plugins` installation
flow. No plugin is therefore claimed as installed.

## Skill selection

| Skill | Status | Direct contribution |
|---|---|---|
| accessibility | Active | WCAG 2.2 AA audit and test matrix |
| animation-vocabulary | Active | exact Loop/Pulse/Shimmer/Fade in/Scale in terminology |
| apple-design | Active | restrained, causal, interruptible motion and hierarchy |
| context7 | Active | current OTel Go, Fiber v2 middleware, and Asynq documentation |
| emil-design-eng | Active | Before/After/Why review and UI-polish standard |
| find-skills | Active | marketplace/adoption and overlap review |
| frontend-design | Active | information architecture, hierarchy, onboarding direction |
| improve-animations | Active | read-only motion audit and standalone implementation plans |
| openai-docs | Active | current official skill/plugin lifecycle and boundary |
| playwright | Installed and verified | real-browser guest/auth accessibility snapshot |
| security-best-practices | Active | structured Go/JavaScript security report |
| security-threat-model | Active | trust-boundary workflow; awaiting assumption confirmation |
| web-design-guidelines | Active | forms, controls, navigation, responsive behavior, reduced motion |

The requested design-report, system-design, and strategy-memorandum template
skills were not present in the active skill catalog. No connected document
session was available. Their intended artifact structures were reproduced in
repository-native Markdown in the transformation blueprint.

## Marketplace evidence

The current skills.sh leaderboard showed strong adoption for the already active
`find-skills`, `frontend-design`, `web-design-guidelines`,
`emil-design-eng`, and browser-testing skills. No Go/OpenTelemetry-specific
skill with a clearer advantage over Context7 plus primary documentation was
identified.

Rejected additions:

| Candidate | Reason |
|---|---|
| `webapp-testing` | overlaps the installed official Playwright workflow |
| platform-specific observability skills | Argus is self-hosted and the target design is vendor-neutral |
| Sentry-specific tooling | no Sentry architecture decision was made |
| generic UI mega-skills | overlap the explicit design/accessibility skills already active |

## Plugin recommendations

These are optional workflow integrations, not runtime dependencies:

1. **Figma** (`figma@openai-curated-remote`)
   - useful after Product/Design approves the information architecture;
   - turns text wireframes into an interactive prototype and supports
     component-state/accessibility handoff;
   - not required for implementation or architecture review.

2. **Atlassian Rovo** (`atlassian-rovo@openai-curated-remote`)
   - useful if Jira/Confluence is the team’s authoritative Scrum workspace;
   - can transfer Epics, stories, acceptance criteria, ADRs, and Sprint goals;
   - should not be connected until the target Jira project and data policy are
     known.

GitHub and Context7 are already active and sufficient for repository delivery
and current technical documentation. Slack, email, Drive, and document-store
plugins are not justified without an explicit notification or document
workflow.

## Installation policy for future additions

Before installing:

- confirm a missing capability, not mere convenience;
- prefer official/curated and high-reputation sources;
- inspect the full skill or plugin manifest, scripts, hooks, and MCP endpoints;
- record version/source/install count and owner reputation;
- avoid hooks or broad credentials unless necessary;
- pin or otherwise make updates reviewable;
- start a new Codex session after plugin/skill installation;
- remove unused capabilities.
