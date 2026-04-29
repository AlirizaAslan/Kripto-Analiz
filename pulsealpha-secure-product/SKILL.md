---
name: pulsealpha-secure-product
description: Build, review, and extend the PulseAlpha real-time market intelligence platform from its PRD. Use when Codex needs to design features, write code, review architecture, create APIs, WebSocket flows, AI prediction UX, or security controls for PulseAlpha while preserving its constraints: decision-support only, no trading automation, no investment-advice framing, strict auth/RBAC, OWASP-focused hardening, data minimization, and safe handling of financial, user, and model-serving workflows.
---

# PulseAlpha Secure Product

Implement PulseAlpha as a real-time market decision-support platform, not as an auto-trading or brokerage system.

## Non-negotiable product boundaries

- Keep the product framed as probabilistic market decision support.
- Do not present signals as guarantees, instructions, or personalized investment advice.
- Do not add automatic trade execution, brokerage order routing, or hidden "one-click trade" shortcuts unless the user explicitly changes the PRD.
- Keep disclaimers visible in user-facing financial signal surfaces.
- Preserve explainability, confidence, and historical performance context around predictions.

## Core stack from the PRD

- Frontend: SvelteKit + TypeScript + TailwindCSS.
- Low-latency backend/orchestration: Go.
- ML training and inference service: Python microservice.
- Relational data: PostgreSQL.
- Time-series data: TimescaleDB or ClickHouse.
- Cache / rate-limit / pub-sub: Redis.
- Streaming/event bus: NATS or Kafka.
- Observability: Prometheus, Grafana, Loki, OpenTelemetry.

If the user requests a major stack change, note the deviation and preserve the same latency, security, and observability properties.

## Security baseline

Apply these controls by default in design, implementation, and review work:

- Enforce TLS for every external and internal network hop where applicable.
- Hash passwords with `argon2` or `bcrypt`; never store reversible credentials.
- Use short-lived access tokens plus refresh-token rotation.
- Apply RBAC for `user`, `admin`, and any elevated operational roles.
- Validate and sanitize all request input, query params, path params, headers, uploaded content, and webhook payloads.
- Add endpoint-specific rate limiting, especially for auth, alerts, and public market-data APIs.
- Log security-relevant actions in audit logs without storing secrets, raw tokens, or unnecessary PII.
- Default to least privilege for services, DB roles, queues, buckets, and admin routes.
- Prevent OWASP Top 10 issues, especially broken access control, injection, SSRF, XSS, CSRF where relevant, insecure deserialization, and security misconfiguration.

## PulseAlpha-specific hardening rules

### Auth and session handling

- Require verified email before enabling sensitive user actions when the flow depends on identity.
- Store refresh tokens hashed or otherwise server-verifiable without exposing raw long-lived tokens broadly.
- Revoke/rotate refresh tokens on suspicious activity, logout, password reset, and credential changes.
- Protect admin endpoints with explicit authorization checks on every request.

### WebSocket and streaming safety

- Authenticate WebSocket connections.
- Authorize subscriptions server-side; never trust client-supplied `userId`, role, or channel ownership.
- Scope private channels to the authenticated principal.
- Rate-limit connection attempts and subscription churn.
- Reject oversized or malformed messages early.
- Avoid leaking internal topology, raw exception traces, or provider credentials through socket events.

### Alerts and outbound integrations

- Treat webhook URLs as sensitive.
- For Telegram/Discord/webhook features, prevent SSRF with allowlists, protocol restrictions, DNS/IP validation, and blocked access to loopback/private metadata endpoints.
- Add cooldown/deduplication to stop alert spam and abuse.
- Avoid putting secrets or internal identifiers into client-visible notification payloads.

### Market and model data handling

- Validate upstream market data before using it for feature engineering or UI display.
- Handle duplicates, missing ticks, outliers, and clock skew explicitly.
- Version models and support rollback.
- Do not load untrusted serialized model artifacts.
- Keep model-serving interfaces narrow and typed; validate feature payloads between Go and Python services.

### Frontend safety

- Escape/sanitize any rich text or explanation content rendered in the UI.
- Do not expose admin-only fields, internal health details, raw stack traces, or provider diagnostics to normal users.
- Keep auth tokens out of URLs and avoid persistent storage choices that expand XSS blast radius unless required by the repo's established auth design.

### Data governance

- Minimize stored personal data.
- Keep retention policies explicit for audit logs, notifications, and model/market history.
- Support KVKK/GDPR-aligned deletion/export workflows if user-account features are implemented.
- Do not copy secrets into fixtures, screenshots, docs, or tests.

## Product implementation priorities

When asked to build features, bias toward the MVP in this order:

1. Auth and email verification.
2. Live asset list and WebSocket market feed.
3. Symbol detail page and live charts.
4. Prediction card with probabilities, confidence, risk, and explanation.
5. Watchlists.
6. Alerts and in-app notifications.
7. Signal history and performance views.
8. Admin health panel.

Keep V2 items behind clear boundaries. Do not silently introduce out-of-scope systems such as automated execution or brokerage coupling.

## Prediction UX rules

- Show `up`, `down`, and `neutral` probabilities together when available.
- Show confidence separately from direction.
- Show risk labels and recent comparable-signal performance where possible.
- Use wording such as "probability", "signal", "confidence", and "historical hit rate".
- Avoid wording such as "guaranteed", "safe profit", "sure win", or direct buy/sell instructions.

## API and schema guidance

- Keep REST endpoints resource-scoped and authorization-aware.
- Use server-derived identity for watchlists, alerts, and private history access.
- Index time-series and prediction queries for symbol + timestamp access patterns.
- Store explainability payloads in structured JSON with explicit schema/versioning.
- For admin retrain or model-management endpoints, require elevated role checks, audit logging, idempotency protection where needed, and abuse controls.

## Review checklist

When reviewing or generating code for PulseAlpha, check for:

- Missing authorization on watchlist, alert, admin, or history endpoints.
- JWT misuse, long-lived tokens, or absent refresh rotation.
- WebSocket channel escalation or cross-user data leaks.
- XSS in explanation, notification, or admin-log rendering.
- SSRF in webhook integrations.
- Missing rate limits on auth or alert endpoints.
- Secret leakage in logs, responses, config, or tests.
- Overstated financial claims or missing disclaimer placement.
- Unbounded retries, reconnect storms, or missing circuit-breaker behavior.
- Missing observability on inference latency, feed delay, drift, and provider failures.

## Working style

- Prefer incremental, observable delivery.
- When requirements are ambiguous, choose the option that reduces security and regulatory risk.
- If the user asks for a risky shortcut, state the risk and implement the safest viable version unless explicitly told otherwise.
- Keep outputs concise and operational.

## References

- Read [references/pulsealpha-constraints.md](references/pulsealpha-constraints.md) when you need the condensed PRD, MVP scope, architectural boundaries, and security/regulatory reminders for implementation work.
