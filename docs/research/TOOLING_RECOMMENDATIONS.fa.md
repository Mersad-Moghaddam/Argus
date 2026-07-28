# ارزیابی مهارت‌ها و پلاگین‌های مناسب برای تحول Argus

تاریخ: ۲۰۲۶-۰۷-۲۸
اصل انتخاب: کمترین ابزار لازم، منبع معتبر، اثر مستقیم بر Product Goal و حداقل ریسک زنجیره تأمین

## نتیجه

- مهارت رسمی **Playwright** از مخزن `openai/skills` با موفقیت در `/home/mersad/.codex/skills/playwright` نصب شد.
- مهارت‌های لازم برای این ممیزی از قبل موجود بودند؛ نصب نسخه‌ی تکراری انجام نشد.
- پلاگین‌های مناسب مرحله‌ی بعد **Figma** و **Atlassian Rovo** هستند، اما در این نشست ابزار Plugin Management قابلیت search/suggest install را callable نکرد؛ بنابراین هیچ نصب پلاگینی تأیید یا ادعا نمی‌شود.
- سه مهارت `openai-templates:artifact-template-*` که در درخواست نام برده شده بودند در catalog فعال این نشست موجود نبودند؛ ساختارهای design report، system-design و strategy memorandum به‌صورت دستی و repo-native در همین بسته بازسازی شدند.

## مهارت‌های استفاده‌شده

| مهارت | اثر واقعی بر خروجی |
|---|---|
| `accessibility` | معیارهای WCAG 2.2، keyboard/focus، form/table/chart و contrast |
| `animation-vocabulary` | نام‌گذاری دقیق Loop، Pulse، Shimmer، Fade in، Scale in و Press/Tap feedback |
| `apple-design` | direct manipulation، motion restraint، gesture/focus و reduced motion |
| `context7:context7-mcp` | مستندات جاری OTel Collector و Fiber security middleware |
| `emil-design-eng` | جدول Before/After/Why و ارزیابی polish/motion |
| `find-skills` | بررسی بازار مهارت‌ها و اجتناب از نصب کم‌اعتبار |
| `frontend-design` | معماری اطلاعات و جهت بصری غیرقالبی |
| `improve-animations` | audit read-only و پنج plan مستقل در `animation-plans/` |
| `openai-docs` | راهنمای رسمی جاری Codex برای تفاوت skill/plugin و lifecycle نصب |
| `web-design-guidelines` | checklist رابط وب، interaction و فرم |
| `security-best-practices` | گزارش ساختاری Go/JavaScript با evidence |
| `security-threat-model` | trust boundary، assets، abuse path و risk calibration |
| `skill-installer` | نصب امن مهارت رسمی Playwright |

## مهارت نصب‌شده: Playwright

### چرا انتخاب شد؟

- منبع رسمی `openai/skills`؛
- کاربرد مستقیم در تست فلو signup/login/project wizard؛
- امکان keyboard، accessibility tree، responsive و request-count verification؛
- مکمل audit ایستا، نه تکرار آن؛
- install و اعتبار اجتماعی بالاتر از گزینه‌های ناشناس بررسی‌شده.

### وضعیت

```text
Installed playwright to /home/mersad/.codex/skills/playwright
```

طبق lifecycle Codex، skill تازه‌نصب‌شده در session جدید فعال می‌شود. بنابراین این ممیزی از آن برای اجرای مرورگر استفاده نکرده و چنین ادعایی ندارد.

### کاربرد پیشنهادی پس از implementation

- signup → verification/login → new project؛
- deep-link returnTo؛
- import OpenAPI و اثبات zero outbound probe؛
- keyboard-only modal/wizard/table sort؛
- axe/accessibility assertions؛
- dark/light و 320px/200% zoom؛
- شمارش requestهای UI و hidden-tab pause؛
- session storage check: عدم وجود credential در local/session storage.

## پلاگین‌ها

### ۱. Figma — پیشنهاد مرحله‌ی طراحی

کاربرد:

- تبدیل wireframeهای متنی به prototype تعاملی؛
- تعریف component stateها و tokens؛
- handoff و annotation دسترس‌پذیری؛
- usability test قبل از تغییر frontend.

چرا اکنون blocker نیست؟

تمام تصمیم‌ها و wireframeها repo-native و قابل review هستند. اتصال Figma زمانی ارزش دارد که owner فایل/فضای طراحی و workflow handoff مشخص باشد.

### ۲. Atlassian Rovo — پیشنهاد مرحله‌ی delivery

کاربرد:

- انتقال Epic/Story/Acceptance Criteria به Jira؛
- لینک ADR، threat model و sprint outcome؛
- traceability از Product Goal تا release gate.

چرا اکنون blocker نیست؟

backlog کامل در Git versioned است. بدون project key، workflow و مالک Jira، ایجاد خودکار ticket می‌تواند duplicate و آشفتگی بسازد.

### پلاگین‌های بررسی‌شده اما نامرتبط با این فاز

| پلاگین | تصمیم |
|---|---|
| Google Drive / Notion / Box / SharePoint | نصب نشود؛ منبع حقیقت فعلاً repository است |
| Slack / Teams | نصب نشود؛ ارسال پیام یا هماهنگی تیم درخواست نشده |
| Gmail / Outlook Email | نصب نشود؛ workflow ایمیل در scope نیست |
| Google/Outlook Calendar | نصب نشود؛ برنامه‌ی Scrum به calendar mutation نیاز ندارد |

## بررسی مهارت‌های جایگزین

### گزینه‌های دیده‌شده

- `openai/skills/playwright`: انتخاب شد.
- مهارت‌های accessibility شخص ثالث: مفید، اما skill داخلی accessibility و audit repo برای این مرحله کافی بود.
- Playwright Pro و test-generatorهای شخص ثالث: قابلیت بالا، اما dependency و instruction surface بیشتر؛ پیش از audit زنجیره تأمین لازم است.

### قواعد زنجیره تأمین

پیش از نصب هر skill/plugin آینده:

1. منبع، owner و commit/tag دقیق ثبت شود.
2. `SKILL.md`، scripts، hooks و network behavior کامل review شود.
3. install count/star صرفاً signal است، نه اثبات امنیت.
4. حداقل permission و sandbox حفظ شود.
5. مهارت تکراری نصب نشود.
6. نسخه یا source revision در تصمیم تیم ثبت شود.
7. پس از نصب، session جدید و smoke test بدون secret انجام شود.

## محدودیت Plugin Management در این نشست

روش درست Codex برای پلاگین غایب، search و سپس suggestion تعاملی است. ابزار این نشست metadata عمومی Plugin Management را نشان داد، اما عملیات `search_plugins`/`suggest_plugins` قابل فراخوانی نشد. چون پیشنهاد نصب نیازمند click/connection صریح کاربر است، هیچ مسیر جایگزین یا نصب ادعایی انجام نشد.

اقدام بعد:

1. در Codex CLI دستور `/plugins` یا Plugin Directory را باز کنید.
2. ابتدا `Figma (figma@openai-curated-remote)` را برای design handoff نصب/متصل کنید.
3. اگر Jira/Confluence واقعاً منبع delivery است، `Atlassian Rovo (atlassian-rovo@openai-curated-remote)` را اضافه کنید.
4. session جدید بسازید تا bundled skill/toolها فعال شوند.

## منابع

- [Codex skills](https://developers.openai.com/plugins/concepts/skills)
- [Codex plugins](https://learn.chatgpt.com/docs/plugins)
- [Playwright skill در skills.sh](https://www.skills.sh/openai/skills/playwright)
- [مخزن رسمی openai/skills](https://github.com/openai/skills)
