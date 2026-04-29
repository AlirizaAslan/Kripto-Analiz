# PulseAlpha Constraints

## Product position

- PulseAlpha is a real-time market decision-support platform.
- It is not an investment-advice engine.
- It is not an order-routing, brokerage, or auto-trading product.

## MVP scope

- Email registration, login, verification, password reset.
- JWT/session auth with refresh rotation.
- Live asset list for stocks and crypto.
- WebSocket-driven live price updates.
- Charts and multiple timeframes.
- AI direction card with probability, confidence, volatility, risk, and explanation.
- Watchlists.
- Alerts and in-app notifications.
- Signal history and success analytics.
- Admin health panel.

## Preferred architecture

- SvelteKit frontend.
- Go API gateway and low-latency services.
- Python inference service.
- PostgreSQL for relational data.
- TimescaleDB or ClickHouse for market time-series.
- Redis for cache/session/rate-limit/pub-sub.
- NATS or Kafka for internal event streaming.

## Security and compliance

- TLS 1.2+ everywhere applicable.
- RBAC and audit logs.
- OWASP Top 10 protections.
- Input validation and sanitization.
- Password hashing with bcrypt or argon2.
- Rate limiting per endpoint.
- KVKK/GDPR-aligned data handling.
- Clear financial disclaimer in user-facing signal flows.

## Reliability goals

- P95 API response under 200 ms.
- P95 real-time update delay under 1 second.
- Inference under 500 ms.
- Graceful degradation on provider failure.
- Retry/failover/circuit-breaker patterns for external dependencies.

## Open decisions from the PRD

- Exact market-data providers.
- Initial exchanges and geography.
- Final prediction horizons if user-configurable.
- Whether order-book features are in MVP or V2.
- Notification channels included in MVP.
- Model training granularity and pricing model.
