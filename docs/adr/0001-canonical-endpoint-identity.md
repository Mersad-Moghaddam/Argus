# ADR 0001: Canonical endpoint identity

## Status

Accepted — 2026-07-28

## Decision

Argus normalizes every endpoint through `internal/domain.NormalizeEndpoint`.
The canonical representation contains a normalized method, IDNA ASCII base URL,
route template, canonical identity, and—only for a concrete route—a resolved
fetch target.

The policy is deliberately conservative:

- HTTP and HTTPS absolute base URLs only; userinfo, fragments, base queries,
  controls, backslashes, malformed escapes, and encoded separators are
  rejected.
- Scheme and host are lowercased; a terminal DNS dot and default ports are
  removed; IDNs use `idna.Lookup`.
- Dot segments are removed. Repeated slashes remain significant. Non-root
  trailing slashes are unified for compatibility with Argus's existing route
  uniqueness model.
- Endpoint identity excludes concrete query values. Query and path fixtures are
  synthetic-check configuration, not catalog identity.
- `{parameter}` templates are validated; legacy `:parameter` input is migrated
  to braces. A template identity is never used directly as a network target.

The worker resolves a parameter-substituted route through Go's structured URL
API, not string concatenation, then applies its independent dial-time and
redirect-time SSRF policy. Normalization is not an egress authorization
decision.

## Consequences

Manual create/update, bulk input, OpenAPI import, and worker composition share
the same canonical path today. The authenticated preview endpoint is
`POST /route/normalization/{projectId}`; it returns
stable `code` and `field` values for invalid requests.

The upcoming environment/hash migration will persist the canonical identity
and versioned hash, backfill legacy rows in batches, and report conflicts before
changing uniqueness constraints.
