# Security Policy

## Supported versions

Argus is under active development. Security fixes are applied to the latest revision of the `main` branch; older revisions and forks are not maintained by this project.

## Reporting a vulnerability

Please do not open a public issue for a suspected vulnerability.

Use GitHub's private vulnerability reporting feature on the repository's **Security** tab. If that feature is unavailable, contact the repository owner privately through their GitHub profile and request a secure reporting channel. Include:

- the affected revision or commit;
- the vulnerable component and configuration;
- reproduction steps or a minimal proof of concept;
- the expected impact;
- any suggested mitigation;
- whether the issue is already public.

Avoid accessing data that is not yours, disrupting services, running destructive tests, or publishing details before a fix is available.

## Response process

The maintainer will aim to acknowledge a complete report within seven days, validate and prioritize it, coordinate a remediation, and credit the reporter when requested. Timelines depend on impact and maintainer availability.

## Deployment responsibilities

Argus initiates outbound network requests and stores operational data. Operators should:

- terminate TLS at a trusted reverse proxy;
- configure a strong `API_KEY`;
- use unique, strong MySQL and Redis credentials;
- keep MySQL and Redis off the public internet;
- restrict Argus outbound access to intended targets where possible;
- protect backups and monitor access logs;
- update dependencies and container images regularly.

Security behavior and current limitations are documented in the README's **Security model** section.
