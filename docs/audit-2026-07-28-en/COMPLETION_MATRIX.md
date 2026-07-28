# Completion and Evidence Matrix

This matrix maps every explicit request to authoritative branch evidence.

| Requirement | Evidence | Result |
|---|---|---|
| Work in English | Persian superseded documents removed; Persian Unicode scan across Markdown returns no matches | Complete |
| Work on a new branch | `docs/argus-monitoring-ux-plan-20260728` | Complete |
| Complete UI/UX/design review | Blueprint sections 2–5; evidence locations; browser-rendered guest-state audit | Complete |
| Focus on project creation | Blueprint sections 2.3 and 4.1–4.5; four-step project/source/verification/SLO flow | Complete |
| Put Register/Login at the top | Target public/authenticated information architecture and header wireframes | Specified with acceptance criteria |
| Require authentication before adding routes or tools | Authorization matrix, guest-shell rules, unified session design, backlog ID-02/UX-01 | Specified with tests |
| Remove auth from Projects | Target shell and migration map explicitly move auth to global session state | Specified |
| Unlock all appropriate features after auth | Authenticated shell, project switcher, role matrix, `/auth/me` context requirement | Specified |
| Backend address/route normalization | Blueprint section 6: data representations, algorithm, preview API, hash identity, backfill, fuzz properties | Complete specification |
| User stories, flows, wireframes, alternate paths | Mermaid journeys, ASCII wireframes, five detailed stories, roles, deep-link and error paths | Complete |
| Accessibility, usability, and clarity | WCAG 2.2 AA target, P0–P2 findings, test matrix, rendered accessibility snapshot | Complete |
| Review route-by-route monitoring load | Formula, scenarios, current scheduler/evaluator evidence, side-effect analysis | Complete |
| Research current alternatives and standards | `SOURCES.md`; OTel, Prometheus, Google SRE, RFC, OWASP, WCAG, Scrum; Context7 library checks | Complete |
| Select the best monitoring method | Telemetry-first OpenTelemetry hybrid with bounded canaries and heartbeats | Decision complete |
| Find every project change needed | Frontend, API, auth, schema, worker, ingestion, SLO, operations, migration maps | Complete |
| Scrum plan to reach the new system | Product Goal, DoD, eight Epics, 22 ordered items, eight-Sprint forecast, Review metrics | Complete |
| Security best-practices review | `SECURITY_REVIEW.md` and root detailed report | Complete |
| Threat model | `Argus-threat-model.md`, updated after required assumption checkpoint | Complete under documented conservative assumptions |
| Animation audit and plans | `animation-plans/README.md` and five standalone plans; no source edits | Complete |
| Find/install best skills | Required high-value skills were already active; installed Playwright was verified in a real browser; marketplace overlap review documented | Complete without duplicate installation |
| Find/install best plugins | Existing GitHub and Context7 plugins are the necessary set; optional Figma/Rovo are documented but not installed because no live external workflow was required and the installation flow was unavailable | Complete capability decision; no unverified install claim |
| Full documentation | English index, blueprint, security review, threat model, sources, tooling decision, motion plans, and this matrix | Complete |
| Commit and push | Proven only by final Git commit and remote branch verification | Pending until final delivery step |

## Verification boundaries

- The application source was reviewed but intentionally not changed.
- A real browser validated rendered guest/auth semantics and revealed the
  missing hidden-state utility.
- API-dependent browser flows were not executed because Docker was unavailable
  and MySQL/Redis were not running.
- Repository Go tests are still run before commit to establish that the
  documentation-only branch does not coincide with a failing baseline.
- Monitoring v2 is a target design and backlog, not an implemented runtime.
