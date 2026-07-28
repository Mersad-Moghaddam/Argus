# قرارداد نرمال‌سازی URL و Route در Argus

- وضعیت: Proposed specification
- نسخه: 1.0
- تاریخ: ۲۰۲۶-۰۷-۲۸
- هدف: یک خروجی canonical، قابل توضیح، idempotent و مشترک بین create، update، bulk، import و evaluator

## ۱. مشکل فعلی

امروز چند مسیر متفاوت برای URL وجود دارد:

- `internal/domain/route.go:95-120` فقط method را uppercase و path را trim می‌کند، `/` اضافه می‌کند و trailing slash را حذف می‌کند.
- `internal/application/routes.go:105-151` base URL را فقط trim و بدون `/` انتهایی ذخیره می‌کند.
- `internal/domain/entities.go:112-120` برای monitor legacy از `url.ParseRequestURI` و allowlist scheme استفاده می‌کند.
- `internal/worker/route_evaluator.go:434-449` base و path را با string concatenation متصل می‌کند.
- update/import/bulk می‌توانند رفتارهای متفاوت یا خطاهای خاموش بسازند؛ JSON نامعتبر header در `routes.go` به رشته‌ی خالی تبدیل می‌شود.

نتیجه: canonical identity پایدار نیست، collision دیر پیدا می‌شود و UI نمی‌تواند دقیقاً بگوید چه requestی ساخته خواهد شد.

## ۲. اهداف و non-goals

### اهداف

- یک pipeline واحد در backend؛
- idempotence: `N(N(x)) = N(x)`;
- حفظ semantic URL و جلوگیری از rewriteهای مخرب؛
- preview قابل توضیح پیش از save؛
- fingerprint ساختاری برای deduplication؛
- سازگاری با RFC 3986، RFC 9110 و OpenAPI 3.1؛
- migration قابل rollback با collision report.

### non-goals

- normalization یک SSRF control نیست.
- اثبات وجود/سلامت مقصد نیست.
- query parameterها را بدون schema مرتب نمی‌کند.
- path را case-insensitive فرض نمی‌کند.
- repeated slash را خودکار collapse نمی‌کند.
- percent-encoded reserved characterها را خودسرانه decode نمی‌کند.

## ۳. مدل ورودی

به‌جای یک رشته‌ی مبهم:

```json
{
  "method": " get ",
  "baseUrl": " HTTPS://Api.Example.COM:443/v1/ ",
  "pathTemplate": "pets/:petId/",
  "query": [
    {"name": "include", "location": "query", "style": "form", "explode": true}
  ],
  "source": "manual"
}
```

خروجی:

```json
{
  "rawInput": {
    "method": " get ",
    "baseUrl": " HTTPS://Api.Example.COM:443/v1/ ",
    "pathTemplate": "pets/:petId/"
  },
  "normalized": {
    "method": "GET",
    "baseUrl": "https://api.example.com/v1",
    "pathTemplate": "/pets/{petId}",
    "requestUrlPreview": "https://api.example.com/v1/pets/{petId}"
  },
  "changes": [
    {"field": "method", "code": "case_normalized"},
    {"field": "baseUrl", "code": "default_port_removed"},
    {"field": "pathTemplate", "code": "colon_parameter_converted"}
  ],
  "warnings": [],
  "fingerprint": "v1:GET:env_123:/pets/{}"
}
```

## ۴. اصول عمومی preprocessing

برای همه‌ی فیلدهای متنی:

1. decode JSON طبق UTF-8 معتبر؛ invalid UTF-8 رد شود.
2. Unicode به NFC تبدیل شود، مگر فیلدی که پروتکل canonical دیگری الزام کند.
3. فقط whitespace ابتدا/انتها trim شود.
4. NUL، C0/C1 control character و bidi control مشکوک رد شود یا با error مشخص نیازمند تأیید امنیتی باشد.
5. backslash در URL authority/path رد شود؛ به slash تبدیل نشود.
6. raw input برای audit با سیاست retention و redaction نگهداری شود؛ secretها هرگز در raw audit ثبت نشوند.

کد خطا:

```text
invalid_utf8
control_character
backslash_not_allowed
input_too_long
```

## ۵. نرمال‌سازی method

1. trim؛
2. uppercase با ASCII؛
3. validate بر token grammar HTTP؛
4. برای catalog می‌توان method استاندارد OpenAPI را پذیرفت؛ extension method تنها با feature flag.

Allowlist اولیه:

```text
GET HEAD OPTIONS POST PUT PATCH DELETE
```

`TRACE` به دلیل ریسک و ارزش کم در محصول عادی رد می‌شود. `CONNECT` پشتیبانی نمی‌شود.

method سه property جدا دارد:

| method | safe | idempotent | synthetic default |
|---|---|---|---|
| GET | بله | بله | قابل پیشنهاد |
| HEAD | بله | بله | قابل پیشنهاد |
| OPTIONS | بله | بله | فقط با دلیل |
| PUT | خیر | بله | خاموش |
| DELETE | خیر | بله | خاموش |
| POST | خیر | معمولاً خیر | خاموش |
| PATCH | خیر | معمولاً خیر | خاموش |

idempotent بودن مجوز probe نیست؛ PUT و DELETE همچنان state-changing هستند.

## ۶. نرمال‌سازی Base URL

### اعتبارسنجی

- parse با parser استاندارد `net/url`، نه regex؛
- scheme فقط `http` یا `https`؛
- absolute URL و host الزامی؛
- userinfo (`user:pass@`) ممنوع؛
- query و fragment ممنوع؛
- port باید عددی و 1..65535؛
- hostname خالی، wildcard، trailing ambiguity یا invalid IDNA رد؛
- IP literal canonical شود؛ IPv6 داخل bracket.

### canonicalization

1. scheme lowercase؛
2. host:
   - Unicode domain با IDNA2008/UTS#46 policy مشخص به ASCII A-label؛
   - lowercase؛
   - یک trailing dot طبق policy حذف و warning ثبت شود؛
3. default port:
   - `http:80` حذف؛
   - `https:443` حذف؛
4. path prefix:
   - path خالی → `/` در parse داخلی؛
   - در storage base، root به `https://host` نمایش داده شود؛
   - dot segmentها با RFC algorithm حذف شوند، اما اگر percent-decoded dot segment پنهان وجود دارد ورودی رد شود؛
   - repeated slash حفظ شود و warning داده شود؛
   - trailing slash storage حذف شود، مگر semantic policy صریح محیط؛
5. percent encoding:
   - hex uppercase؛
   - فقط unreservedها (`ALPHA DIGIT - . _ ~`) decode شوند؛
   - reservedها encoded باقی بمانند؛
   - invalid escape رد شود.

### مثال

| ورودی | canonical | نتیجه |
|---|---|---|
| ` HTTPS://Api.Example.COM:443/ ` | `https://api.example.com` | معتبر |
| `http://example.com:80/api/` | `http://example.com/api` | معتبر |
| `https://user:pass@example.com` | — | `userinfo_not_allowed` |
| `https://example.com/api?q=1` | — | `base_query_not_allowed` |
| `https://example.com/#x` | — | `base_fragment_not_allowed` |
| `file:///etc/passwd` | — | `unsupported_scheme` |
| `https://example.com/%2e%2e/admin` | — | `encoded_dot_segment` |
| `https://EXÄMPLE.org` | `https://xn--exmple-...` | پس از IDNA معتبر |

IDNA example نهایی باید با library منتخب و test vector رسمی قفل شود؛ خروجی نمونه‌ی بالا عمداً abbreviated است و نباید golden test تلقی شود.

## ۷. نرمال‌سازی Path Template

### grammar

- با `/` شروع شود؛ در ورودی convenience، `/` اضافه و change ثبت می‌شود.
- scheme/authority/query/fragment نداشته باشد.
- طول پیش‌فرض ≤ 1024 byte بعد از UTF-8؛ قابل تنظیم با سقف سخت.
- path case حفظ شود.
- segment خالی ناشی از `//` حفظ و warning داده شود.
- trailing slash برای identity حذف شود، جز root.

### template parameters

فرم canonical:

```text
/pets/{petId}/owners/{ownerId}
```

قواعد:

- `:petId` فقط وقتی کل segment است به `{petId}` تبدیل شود.
- `{name}` باید balanced و کل segment باشد؛ mixed segment مثل `file-{id}` در v1 رد یا به‌عنوان literal ثبت نمی‌شود.
- نام پارامتر: `[A-Za-z_][A-Za-z0-9_.-]*` و length محدود.
- duplicate parameter name مجاز نیست مگر OpenAPI policy صریح.
- parameter value در canonical template وجود ندارد.
- encoded brace برای فرار از validation استفاده نمی‌شود.

### dot/escape

- literal `.` segment می‌تواند طبق RFC حذف شود، اما برای وضوح ورودی کاربر warning شدید؛
- `..` و انواع percent-encoded آن رد می‌شوند؛
- `%2F` و `%5C` در template به‌طور پیش‌فرض رد می‌شوند چون proxy/serverها decoding متفاوت دارند؛
- percent encoding invalid یا double-encoded ambiguity (`%252F`) warning/error policy دارد.

### مثال

| ورودی | canonical | تغییر/خطا |
|---|---|---|
| `pets/:id/` | `/pets/{id}` | slash + colon + trailing |
| `/Pets/{id}` | `/Pets/{id}` | case حفظ |
| `/pets/{name}` | `/pets/{name}` | معتبر |
| `/pets/{id}?full=1` | — | query باید structured باشد |
| `/a//b` | `/a//b` | warning repeated slash |
| `/a/../admin` | — | dot segment traversal |
| `/files/%2Fetc` | — | encoded slash ambiguity |

## ۸. Query و header

### Query

query بخشی از endpoint identity نیست مگر product requirement صریح؛ به‌صورت structured parameter ذخیره می‌شود:

```json
{
  "name": "tag",
  "in": "query",
  "style": "form",
  "explode": true,
  "required": false,
  "example": ["one", "two"]
}
```

- ترتیب repeated query ممکن است semantic باشد؛ sort کور ممنوع.
- secret query value ممنوع یا در encrypted secret reference.
- UI باید template و fixture را جدا نشان دهد.

### Header

- JSON نامعتبر error است؛ به مقدار خالی تبدیل نمی‌شود.
- نام header با `http.CanonicalHeaderKey` تنها برای display؛ identity case-insensitive.
- hop-by-hop و transport-owned headerها رد می‌شوند:
  `Host`, `Content-Length`, `Connection`, `Transfer-Encoding`, `Proxy-*`.
- secret value در encrypted reference، نه plaintext JSON عمومی.
- newline/CRLF در name/value رد.

## ۹. اتصال Base و Path

string concatenation ممنوع. Join باید segment-aware باشد.

فرض:

```text
base path prefix = /v1
endpoint template = /pets/{id}
result = /v1/pets/{id}
```

اگر endpoint از OpenAPI server URL با prefix آمده، parser باید provenance را نگه دارد تا prefix دوبار اعمال نشود.

قواعد:

- exactly one boundary slash؛
- slashهای داخلی حفظ؛
- هیچ path نمی‌تواند authority را override کند؛
- fixture parameter با `url.PathEscape` و segment-safe policy جایگزین شود؛
- پس از substitution دوباره URL parse و security validation شود.

## ۱۰. Fingerprint و duplicate

### display identity

```text
environment_id + method + canonical_path_template
```

### structural identity

OpenAPI path `/pets/{id}` و `/pets/{name}` از نظر match یکسان‌اند. fingerprint نام variable را حذف می‌کند:

```text
v1 | environment_id | GET | /pets/{}
```

hash اختیاری:

```text
SHA-256(version || NUL || environment_id || NUL || method || NUL || structural_template)
```

نسخه‌ی normalization داخل fingerprint الزامی است تا migration آینده قابل مدیریت باشد.

Collision:

- same structural identity و metadata متفاوت → update/conflict preview؛
- method متفاوت → endpoint جدا؛
- environment متفاوت → endpoint جدا؛
- source متفاوت به‌تنهایی endpoint جدا نمی‌سازد؛ provenance چندگانه دارد.

## ۱۱. API قرارداد

### Preview

```http
POST /api/v2/projects/{projectId}/endpoints:normalize
Content-Type: application/json
```

Response success:

```json
{
  "valid": true,
  "normalizationVersion": "1",
  "normalized": {
    "method": "GET",
    "baseUrl": "https://api.example.com/v1",
    "pathTemplate": "/pets/{petId}",
    "structuralTemplate": "/pets/{}"
  },
  "changes": [],
  "warnings": [],
  "fingerprint": "v1:..."
}
```

Response invalid:

```json
{
  "valid": false,
  "errors": [
    {
      "field": "baseUrl",
      "code": "base_fragment_not_allowed",
      "message": "Base URL cannot contain a fragment.",
      "range": {"start": 23, "end": 27}
    }
  ]
}
```

### Save

Save همان service را دوباره اجرا می‌کند؛ client preview قابل اعتماد فرض نمی‌شود. response شامل normalized object و `normalizationVersion` است.

### Concurrency

- `Idempotency-Key` برای create/bulk commit؛
- `If-Match`/version برای update؛
- duplicate fingerprint پاسخ `409` با existing endpoint reference و actionهای مجاز.

## ۱۲. Package design پیشنهادی Go

```text
internal/domain/target/
├── method.go
├── base_url.go
├── path_template.go
├── fingerprint.go
├── result.go
└── errors.go

internal/application/
└── endpoint_normalization.go
```

API مفهومی:

```go
type NormalizeInput struct {
    Method       string
    BaseURL      string
    PathTemplate string
    EnvironmentID int64
}

type NormalizeResult struct {
    Method             string
    BaseURL            string
    PathTemplate       string
    StructuralTemplate string
    Fingerprint        string
    Version            string
    Changes            []Change
    Warnings           []Warning
}

func NormalizeEndpoint(input NormalizeInput, policy Policy) (NormalizeResult, error)
```

تابع domain pure است؛ DNS/network نمی‌زند. security validator در evaluator جدا و در زمان dial اجرا می‌شود.

## ۱۳. Migration و backfill

### مرحله ۱ — Shadow columns

افزودن:

```text
raw_base_url
canonical_base_url
raw_path_template
canonical_path_template
structural_fingerprint
normalization_version
normalization_status
```

### مرحله ۲ — Dry-run

- batch bounded بر ID؛
- بدون overwrite؛
- report:
  - valid unchanged
  - valid changed
  - invalid
  - collision
  - security-policy rejected

### مرحله ۳ — Review collisions

- alias table برای raw identityهای قدیمی؛
- merge تنها با تصمیم deterministic/کاربر؛
- check history به endpoint برنده re-link؛
- هیچ delete خودکار.

### مرحله ۴ — Dual-read/write

- write فقط canonical v1؛
- read ابتدا canonical، fallback legacy؛
- metric برای fallback rate.

### مرحله ۵ — Constraint

- unique index بر `(environment_id, method, structural_fingerprint)`;
- not-null پس از صفرشدن invalid backlog؛
- حذف fallback در release بعد.

Rollback:

- raw columns حفظ؛
- feature flag resolver؛
- migration destructive تا پایان retention ممنوع.

## ۱۴. تست

### Unit/property

- idempotence؛
- no panic برای arbitrary UTF-8/bytes؛
- canonical parse→serialize stability؛
- reserved character preservation؛
- IDNA test vector؛
- IPv4/IPv6 canonical؛
- dot/encoded traversal؛
- variable-name structural collision؛
- case preservation path؛
- repeated slash policy.

### Fuzz

اهداف:

- base URL parser؛
- path template parser؛
- percent decoder؛
- join/substitution؛
- header JSON validator.

Invariantها:

- خروجی control character ندارد؛
- خروجی base absolute HTTP(S) است؛
- join نمی‌تواند host را تغییر دهد؛
- secret وارد message/log نمی‌شود.

### Integration

- preview و save خروجی یکسان؛
- manual/bulk/import canonical یکسان؛
- update collision atomic؛
- backfill resumable؛
- evaluator تنها canonical target را می‌گیرد و dial-time SSRF check دارد.

## ۱۵. Observability

Metrics:

```text
argus_normalization_total{result,source,version}
argus_normalization_warning_total{code}
argus_normalization_error_total{code}
argus_normalization_collision_total{source}
argus_normalization_fallback_total{version}
argus_normalization_duration_seconds
```

ممنوع: raw URL/host/path به‌عنوان metric label. log نمونه‌برداری‌شده با hash/fingerprint و tenant ID؛ raw فقط در audit store امن و redacted.

## ۱۶. معیار پذیرش

- همه‌ی entry pointها یک service واحد را صدا می‌زنند.
- preview/save parity صددرصد در integration test.
- property test idempotence پاس.
- collision report پیش از unique constraint تولید و review.
- هیچ fragment/userinfo/control/encoded traversal ذخیره نمی‌شود.
- evaluator با string concatenation URL نمی‌سازد.
- UI تمام تغییرهای معنی‌دار را قبل از save نمایش می‌دهد.
- normalization version در داده و fingerprint ثبت می‌شود.

## منابع

- [RFC 3986 — URI Generic Syntax](https://www.rfc-editor.org/rfc/rfc3986.html)
- [RFC 9110 — HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110.html)
- [RFC 6570 — URI Template](https://www.rfc-editor.org/info/rfc6570/)
- [OpenAPI Specification 3.1.1 — Paths](https://spec.openapis.org/oas/v3.1.1.html)
- [Go `net/url`](https://pkg.go.dev/net/url)
- [OWASP SSRF Prevention](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html)
