# Product Backlog و نقشه‌ی Scrum برای Argus Monitoring v2

- تاریخ: ۲۰۲۶-۰۷-۲۸
- افق برنامه‌ریزی: ۸ Sprint دو‌هفته‌ای برای forecast؛ scope هر Sprint در Sprint Planning با ظرفیت واقعی بازتنظیم می‌شود.
- مبنا: `docs/architecture/MONITORING_V2_ADR.fa.md`
- چارچوب: Scrum Guide 2020؛ Product Goal واحد، Increment قابل استفاده در پایان هر Sprint و Product Backlog زنده

## ۱. Product Goal

> کاربران Argus بتوانند سلامت همه‌ی endpointهای API خود را عمدتاً از تله‌متری واقعی و بدون ایجاد بار مصنوعی متناسب با تعداد routeها مشاهده کنند؛ تنها canaryهای محدود، امن و بودجه‌بندی‌شده request فعال بسازند؛ و incidentها بر اساس SLO قابل توضیح، tenant-safe و قابل اعتماد ایجاد شوند.

### Outcomeهای قابل اندازه‌گیری

تا پایان goal:

- تعداد synthetic request نسبت به baseline پروژه‌های مهاجرت‌کرده حداقل ۹۰٪ کاهش یابد.
- import هر OpenAPI به‌تنهایی دقیقاً صفر active probe ایجاد کند.
- unsafe synthetic در production به‌صورت پیش‌فرض صفر باشد.
- telemetry-to-dashboard p99 lag زیر SLO مصوب تیم باشد.
- cross-tenant access/attribution در تست امنیتی صفر.
- false page rate در pilot نسبت به سیستم فعلی حداقل ۳۰٪ کاهش یابد.
- حداقل ۹۵٪ routeهای پرترافیک pilot با `http.route` canonical map شوند.
- فلو ثبت‌نام تا اولین پروژه median کمتر از دو دقیقه باشد.

## ۲. نقش‌ها و مسئولیت‌ها

| نقش | مسئولیت |
|---|---|
| Product Owner | Product Goal، ordering backlog، acceptance outcome و تصمیم scope |
| Scrum Master | رفع مانع، سلامت eventها، جلوگیری از تبدیل Scrum به waterfall |
| Developers | طراحی، توسعه، تست، امنیت، عملیات و Increment Done؛ مالکیت جمعی |
| SRE/Security/Design | بخشی از Developers یا stakeholder ثابت؛ review بیرونی جای Done را نمی‌گیرد |
| Stakeholders | feedback در Sprint Review: کاربران pilot، on-call، تیم API |

یک نفر Product Owner است. کمیته می‌تواند مشورت دهد اما ordering دوگانه ممنوع است.

## ۳. Definition of Ready

Ready یک gate رسمی Scrum نیست؛ این checklist برای کاهش ambiguity است:

- ارزش کاربر و ارتباط با Product Goal روشن؛
- acceptance criteria قابل تست؛
- threat/privacy impact مشخص؛
- dependency و rollout flag معلوم؛
- mock/wireframe یا API contract در صورت نیاز؛
- telemetry و rollback requirement تعریف‌شده؛
- اندازه‌ی story ترجیحاً ≤ 8 point و قابل تکمیل در یک Sprint؛
- unknown بزرگ به spike time-boxed تبدیل شده است.

## ۴. Definition of Done

هر Product Backlog Item تنها وقتی Done است که:

- code review و تست‌های unit/integration/e2e مرتبط پاس؛
- tenant authorization و negative path تست شده؛
- security checklist و secret/redaction بررسی شده؛
- migration forward/rollback یا N/A مستند؛
- observability خود feature: metric/log/runbook؛
- accessibility برای UI: keyboard، focus، name/role/value، contrast و reduced motion؛
- API/docs/user-facing copy به‌روز؛
- feature flag و rollback تست‌شده؛
- performance/load budget پاس؛
- هیچ Critical/High شناخته‌شده‌ی بدون تصمیم risk acceptance؛
- روی محیط staging deploy و demoپذیر؛
- Increment با main سازگار و قابل عرضه است.

## ۵. Epicها

| ID | Epic | Outcome | وابستگی |
|---|---|---|---|
| E1 | Identity & Tenant Boundary | یک هویت سراسری، منابع project-scoped | پایه |
| E2 | Canonical Endpoint Model | environment و endpoint identity پایدار | E1 |
| E3 | Safe Synthetic Guardrails | import بدون traffic و probe بودجه‌بندی‌شده | E1,E2 |
| E4 | Telemetry Ingestion | OTLP امن تا metrics backend | E1 |
| E5 | SLI/SLO & Incident Engine | alert بر error budget | E2,E4 |
| E6 | Product UX & Onboarding | signup و project/source wizard | E1,E2,E4 |
| E7 | Migration & Operations | shadow، pilot، cutover و rollback | همه |
| E8 | Accessibility & Quality | WCAG 2.2 AA و performance envelope | cross-cutting |

## ۶. Product Backlog

برآوردها story point نسبی هستند و پس از refinement تیم اعتبار می‌گیرند.

### E1 — Identity & Tenant Boundary

#### US-101 — Auth سراسری (8)

به‌عنوان کاربر می‌خواهم از header ثبت‌نام/ورود کنم تا یک session برای همه‌ی ابزارهای خصوصی داشته باشم.

Acceptance:

- logged-out header CTA ثبت‌نام اصلی و login ثانویه دارد.
- private APIها session واحد را می‌پذیرند.
- browser token در `localStorage` نیست.
- cookie production: `HttpOnly`, `Secure`, `SameSite` مصوب و rotation.
- CSRF برای mutationها enforce.
- deep-link returnTo فقط same-origin allowlist.

#### US-102 — Profile & capability contract (5)

- `/api/v2/auth/me` user، active project و capabilityها را از سرور می‌دهد.
- client cached user را authority نمی‌داند.
- permission error با 401/403 درست تفکیک می‌شود.

#### US-103 — Auth abuse controls (8)

- login/register/reset rate limit per-IP + per-account key با privacy.
- generic login error؛ duplicate register flow enumeration-resistant.
- audit event بدون password/token/email plaintext غیرضروری.
- session revoke current/all و expiry.

#### US-104 — Legacy ownership migration (13)

- website/status page/channel/maintenance به project/user متصل‌اند.
- همه‌ی queryها tenant predicate دارند.
- legacy `X-API-Key` deprecation path و metric usage.
- backfill collision/orphan report و rollback.

### E2 — Canonical Endpoint Model

#### US-201 — Environment entity (8)

- project چند environment دارد.
- base URL canonical + raw/audit representation.
- unique name در project و authorization.

#### US-202 — Pure normalization service (13)

- spec `URL_ROUTE_NORMALIZATION_SPEC.fa.md` اجرا شده.
- manual/bulk/import/update یک service واحد.
- idempotence/property/fuzz test.
- change/warning/error machine code.

#### US-203 — Normalize preview API (5)

- preview بدون save؛
- preview/save parity؛
- fingerprint/conflict response؛
- no raw URL metric label.

#### US-204 — Endpoint/policy separation (13)

- endpoint catalog مستقل از monitor policy.
- import endpoint می‌سازد، synthetic را فعال نمی‌کند.
- provenance manual/import/telemetry حفظ.

### E3 — Safe Synthetic Guardrails

#### US-301 — Immediate default-off guardrail (3)

- create/import جدید `synthetic_enabled=false`.
- existing behavior پشت feature flag.
- migration وضعیت current monitors را صریح می‌کند.

#### US-302 — Method safety policy (8)

- GET/HEAD قابل پیشنهاد؛ TRACE/CONNECT ممنوع.
- state-changing production exception نیازمند policy/approval.
- retry method-aware؛ POST بدون idempotency strategy retry نمی‌شود.

#### US-303 — Budgeted scheduler (13)

- token bucket per-origin/project/location.
- jitter، backoff، `Retry-After`، circuit breaker.
- global/tenant concurrency و fair queue.
- queue age/freshness metrics.

#### US-304 — Probe agent for private targets (13، conditional)

- outbound-only agent registration.
- signed short-lived job.
- network scope و secret least privilege.
- revoke/upgrade/health.

#### US-305 — Multi-location quorum (8)

- location health جدا از target health.
- quorum policy و failure class.
- duplicate incident prevention.

### E4 — Telemetry Ingestion

#### SP-401 — Backend selection spike (3)

Time-box: سه روز. benchmark Prometheus-compatible گزینه‌ی MVP با volume forecast، retention، HA، backup و query latency. خروجی ADR فرعی، نه production code بی‌پایان.

#### US-402 — Collector gateway MVP (13)

- OTLP gRPC/HTTP با TLS/auth.
- memory limiter، batch، retry/persistent queue.
- tenant/project attribution server-side.
- invalid/oversized payload rejection.

#### US-403 — Cardinality guard (8)

- attribute allowlist/denylist.
- raw path/query/PII حذف.
- per-tenant series budget و alert.
- hostile payload load test.

#### US-404 — RED ingestion/rollup (13)

- rate/error/duration و last seen.
- `http.route` template mapping.
- rollup idempotent و late/out-of-order policy.

#### US-405 — Telemetry setup UX/API (8)

- credential scoped/rotatable.
- snippets Go/Collector؛ secret فقط یک‌بار.
- waiting/connected/stale state.

### E5 — SLI/SLO & Incidents

#### US-501 — SLO domain & API (13)

- availability و latency indicator.
- target/window/exclusion versioning.
- preview با historical data.

#### US-502 — Burn-rate evaluator (13)

- multi-window rules.
- missing/no-data policy.
- deterministic test با fixture.
- evaluation lag metrics.

#### US-503 — Incident source arbitration (13)

- telemetry، synthetic و heartbeat evidence.
- dedupe و event timeline.
- maintenance/deploy annotations.
- recovery window.

#### US-504 — Notification migration (8)

- v2 incident event با idempotency.
- route/channel scope.
- delivery audit و retry budget.

### E6 — UX & Onboarding

#### US-601 — Public/authenticated shell (8)

- IA هدف سند UX.
- signup/login header.
- project switcher/account menu پس از auth.
- public status exception.

#### US-602 — New project wizard (13)

- Basics → Environment → Source → Review.
- advanced retry/threshold از first step حذف.
- source و requests/min واضح.
- draft/recovery و error focus.

#### US-603 — Import catalog wizard (8)

- normalize/conflict preview.
- zero-probe confirmation.
- safe canary selection مرحله‌ی جدا.

#### US-604 — Health states & explanations (8)

- healthy/degraded/down/no_data/paused/unknown.
- evidence source، freshness و last seen.
- هیچ state فقط با رنگ منتقل نمی‌شود.

#### US-605 — Dashboard request efficiency (8)

- hidden tab pause.
- BFF/delta/ETag.
- SSE فقط برای incident/state transition در صورت توجیه.
- request-count budget e2e.

### E8 — Accessibility & Quality

#### US-801 — Dialog/keyboard system (8)

- native `<dialog>` یا controller کامل.
- focus trap/inert/restore/Escape.
- keyboard test.

#### US-802 — Forms/tables/charts (13)

- name/type/autocomplete/error association.
- sortable button + `aria-sort`.
- chart summary/table.
- live region کوچک.

#### US-803 — Tokens/target/motion (8)

- contrast AA در dark/light.
- target size.
- motion planها اجرا و reduced-motion.
- hover فقط pointer fine.

#### US-804 — Automated quality gates (8)

- axe/Playwright مسیرهای اصلی.
- Go test/race، fuzz budget، dependency/security scans.
- performance/load baseline.

## ۷. Forecast هشت Sprint

هر Sprint باید Increment قابل استفاده داشته باشد. itemهای ناتمام Done نیستند و به backlog برمی‌گردند؛ scope با سرعت واقعی تیم تنظیم می‌شود.

### Sprint 1 — Stop the harm + foundation

Sprint Goal: ایجاد probe ناخواسته متوقف شود و مرز auth v2 قابل آزمایش باشد.

- US-301 default-off؛
- skeleton `/api/v2` و tenant context؛
- US-103 rate limit حداقلی؛
- threat assumptions نهایی؛
- telemetry و baseline outbound request count.

Review demo: import یک spec با ۵۰۰ route و اثبات صفر outbound probe.

### Sprint 2 — Identity and environment

Sprint Goal: کاربر با session واحد یک project/environment امن می‌سازد.

- US-101، US-102؛
- US-201؛
- migration پایه ownership؛
- UI shell اولیه.

Review: signup از header → project/environment؛ بدون localStorage token.

### Sprint 3 — Canonical catalog

Sprint Goal: هر endpoint identity پایدار و preview قابل توضیح داشته باشد.

- US-202، US-203؛
- US-204 بخش model/import؛
- backfill dry-run؛
- fuzz/property tests.

Review: manual/bulk/import ورودی‌های متفاوت را به fingerprint یکسان می‌رسانند و collision قبل از save دیده می‌شود.

### Sprint 4 — Telemetry pipe

Sprint Goal: telemetry pilot به‌صورت امن ingest و قابل مشاهده شود.

- SP-401؛
- US-402، US-403؛
- deployment Compose staging؛
- ingestion dashboards/runbook.

Review: دو tenant هم‌زمان telemetry می‌فرستند؛ attribution درست و cardinality attack محدود است.

### Sprint 5 — RED and onboarding

Sprint Goal: routeهای واقعی از telemetry، بدون probe، health اولیه بگیرند.

- US-404، US-405؛
- US-604 stateها؛
- no-data/freshness؛
- pilot SDK docs.

Review: error/latency injection در sample API روی dashboard endpoint template دیده می‌شود.

### Sprint 6 — SLO and incident shadow

Sprint Goal: SLO v2 incident candidate قابل توضیح اما بدون page واقعی بسازد.

- US-501، US-502؛
- US-503 shadow؛
- historical replay و comparison.

Review: burn scenario deterministic و timeline evidence.

### Sprint 7 — Safe canaries + complete UX

Sprint Goal: critical journey با budget و quorum، در UX نهایی project wizard فعال شود.

- US-302، US-303، US-305؛
- US-602، US-603؛
- US-801/802 مهم‌ترین مسیرها.

Review: review screen requests/min را نشان می‌دهد؛ kill switch و rate cap زیر load اثبات می‌شود.

### Sprint 8 — Pilot cutover

Sprint Goal: cohort pilot با release gate کامل به v2 cutover و rollback تست‌شده برسد.

- US-503/504 تکمیل؛
- US-605، US-803/804؛
- US-104 تکمیل لازم برای cohort؛
- migration، chaos/load/security/a11y؛
- runbook و آموزش on-call.

Review: game day failure، notification، recovery و rollback.

`US-304 private agent` اگر نیاز قطعی MVP باشد باید با item کم‌اولویت دیگری تعویض شود؛ اضافه‌کردن بدون کاهش scope forecast را غیرواقعی می‌کند.

## ۸. Sprint eventها

### Sprint Planning

- Why: Sprint Goal متصل به Product Goal؛
- What: انتخاب itemها بر اساس ظرفیت و Done؛
- How: طرح اولیه Developers، نه task assignment از بالا.

### Daily Scrum

۱۵ دقیقه برای progress به Sprint Goal و تنظیم plan. گزارش وضعیت به مدیر نیست.

### Refinement

یک یا دو جلسه در هفته؛ normalization examples، threat cases، telemetry envelope و UX prototype پیش از Sprint بعد.

### Sprint Review

demo روی staging با داده‌ی واقعی/fixture؛ outcome metric و stakeholder feedback؛ slide-only ممنوع.

### Retrospective

یک اقدام measurable وارد Sprint بعد؛ نمونه: کاهش cycle time migration review یا flaky e2e.

## ۹. تست و Verification Matrix

| سطح | پوشش |
|---|---|
| Domain unit | normalization، method policy، SLO math، state machine |
| Property/fuzz | URL/path/join، OTLP attributes، parser |
| Repository integration | tenant scope، collision، migration resume |
| API integration | auth/CSRF/rate limit/idempotency/error contract |
| Worker integration | fair queue، budget، retry، redirect/DNS security |
| E2E | signup، project wizard، import zero-probe، telemetry connect، incident |
| Accessibility | axe + keyboard + screen reader manual |
| Load | OTLP ingest، series cardinality، scheduler origin caps |
| Chaos | gateway/backend outage، queue recovery، location outage |
| Security | SSRF rebinding/redirect، cross-tenant، token replay، CSRF/XSS |

## ۱۰. Release strategy

Feature flags:

```text
auth_v2
endpoint_normalization_v1
telemetry_ingestion
slo_shadow
synthetic_policy_v2
incident_v2
ui_shell_v2
```

Cohort:

1. internal sample project؛
2. یک self-hosted friendly user؛
3. 5% eligible projects؛
4. 25%؛
5. 100% پس از gate.

در هر مرحله:

- compare old/new status؛
- outbound request count؛
- ingestion lag/drop؛
- incident precision؛
- support feedback؛
- rollback drill.

## ۱۱. Risk register

| ID | ریسک | احتمال/اثر | کنترل | Trigger/Owner |
|---|---|---|---|---|
| R1 | metric cardinality explosion | بالا/بحرانی | allowlist، budget، drop metric | series growth؛ SRE |
| R2 | cross-tenant attribution | متوسط/بحرانی | gateway overwrite، tenant tests | mismatch؛ Security |
| R3 | telemetry no-data به‌جای downtime | بالا/بالا | state جدا + canary | freshness gap؛ Product/SRE |
| R4 | synthetic side effect | متوسط/بحرانی | default-off، safe method، exception control | unsafe policy؛ Security |
| R5 | migration collision | بالا/متوسط | dry-run، aliases، no auto-delete | collision report؛ Backend |
| R6 | Collector operational burden | متوسط/بالا | runbook، queue، capacity test | drop/lag؛ SRE |
| R7 | auth migration locks users out | متوسط/بالا | dual validation موقت، revoke/rollback | 401 spike؛ Backend |
| R8 | schedule forecast overcommit | بالا/متوسط | Sprint scope negotiation، velocity | carry-over؛ Scrum Master |
| R9 | privacy/retention mismatch | نامشخص/بالا | assumption decision + data classification | stakeholder input؛ PO |
| R10 | UI complexity | متوسط/متوسط | progressive disclosure، usability test | task failure؛ Design |

## ۱۲. Dashboard پیشرفت Product Goal

فقط velocity معیار موفقیت نیست. dashboard:

- outbound probe requests/project/min؛
- telemetry coverage درصد endpoint با sample معتبر؛
- mapping success به canonical template؛
- ingest lag/drop/cardinality؛
- SLO candidate precision/recall در fault injection؛
- auth/signup/project funnel؛
- accessibility violations؛
- cycle time و escaped defects؛
- error budget خود Argus.

## ۱۳. تصمیم‌های لازم پیش از Sprint 1

1. آیا deployment هدف single-user self-hosted است یا multi-tenant SaaS نیز در scope نزدیک است؟
2. آیا private target در MVP الزام دارد؟
3. retention و residency requirement چیست؟
4. pilot user و load envelope واقعی کدام است؟
5. team size/skills برای forecast؛ برآورد sprint بدون ظرفیت تعهد نیست.
6. SLO اولیه خود Argus و notification severity policy.

## منبع

- [The Scrum Guide — نسخه رسمی جاری 2020](https://scrumguides.org/scrum-guide.html)
- [Download Scrum Guide](https://scrumguides.org/download.html)
