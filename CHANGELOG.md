# Changelog

## v1.1.0 - 2026-03-23

### Added

- Nuevo `oneclick.Client` como capa raw del contrato Transbank.
- Nuevo `oneclick.FlowService` para flujo alto nivel de inscripción.
- `Config` con defaults, autodetección de ambiente y `Validate()` fail-fast.
- Options pattern (`WithHTTPClient`, `WithLogger`, `WithRetryPolicy`, `WithClock`, `WithBaseURL`, entre otras).
- Resiliencia integrada con retries y circuit breaker.
- Hooks de observabilidad (`BeforeRequest`, `AfterRequest`, `OnError`).
- Métricas internas (`MetricsSnapshot`).
- `StateStore` con implementaciones:
  - in-memory
  - adapter Postgres (`database/sql`)
  - adapter Redis (interfaz `RedisKV`)
- Idempotencia para `FlowService.StartInscription` y `FlowService.ConfirmInscription`.
- Errores tipados SDK (`SDKError`) con `Code()`, `Retryable()`, `UserSafeMessage()`.
- Nuevos tests unitarios de flujo (`oneclick/flow_test.go`).

### Changed

- `OneclickService` queda como alias compatible de `Client`.
- Parseo y wrapping de errores HTTP/Transbank con metadata de retry.
- Documentación actualizada para arquitectura v1.1.0.

### Compatibility

- API legacy (`Start`, `Finish`, `Remove`, `Authorize`, `Status`, `Refund`, `Capture`) se mantiene compatible.
