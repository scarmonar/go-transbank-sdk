# Changelog

## v1.2.1 - 2026-03-23

### Added

- Helper público `ClassifyResponseCode(code)` para traducir `response_code` de Transbank a una clasificación estable y un mensaje apto para UI.
- Cobertura explícita de los rechazos Oneclick documentados por Transbank:
  - `-96`: `tbk_user` no existente
  - `-97`: límite diario de monto excedido
  - `-98`: límite de monto excedido
  - `-99`: límite diario de cantidad de pagos excedido
- Documentación de uso para mostrar al usuario por qué una tarjeta fue rechazada.

### Changed

- Se mantiene el contrato raw existente; el cambio es aditivo y compatible.
- Se actualiza el README e índice con la nueva guía de `response_code`.

### Compatibility

- Release patch aditiva: no se eliminan ni renombran APIs existentes.
- El fallback para códigos no reconocidos sigue siendo seguro para UI.

## v1.2.0 - 2026-03-23

### Added

- `FlowConfirmResponse` expone campos top-level: `BusinessID`, `SubscriptionID`, `Context`.
- Helper público `ClassifyError(err)` con clasificación estable:
  - `validation`
  - `transport`
  - `gateway`
  - `token_not_found`
  - `flow_state`
  - `unknown`
- Nuevos métodos de alto nivel en `FlowService`:
  - `AuthorizeCharge(ctx, FlowAuthorizeChargeRequest)`
  - `ReverseCharge(ctx, FlowReverseChargeRequest)`
- Idempotencia para `flow_authorize_charge` y `flow_reverse_charge` usando el `IdempotencyStore` existente.
- Nuevos tests para clasificación de errores, flujo de cargos/reversas y hooks con correlación por `request_id`.

### Changed

- `request_id` de hooks ahora es consistente por operación y se mantiene entre retries de la misma operación.
- `FlowService.ConfirmInscription` ahora devuelve contexto expandido sin necesidad de lookup manual en `State`.
- Documentación actualizada con ejemplos de:
  - confirmación con contexto expandido
  - `ClassifyError(err)`
  - `FlowService.AuthorizeCharge` y `FlowService.ReverseCharge`
  - ciclo de hooks y correlación

### Compatibility

- Release minor aditiva: no se eliminan ni renombran APIs existentes.
- Se mantiene compatibilidad con wrappers legacy y contratos previos.

## v1.1.0 - 2026-03-23

### Added

- Nuevo `oneclick.Client` como capa raw del contrato Transbank.
- Nuevo `oneclick.FlowService` para flujo alto nivel de inscripción.
- `Config` con defaults, autodetección de ambiente y `Validate()` fail-fast.
- Options pattern (`WithHTTPClient`, `WithLogger`, `WithRetryPolicy`, `WithClock`, entre otras).
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
- `baseURL` del API Oneclick pasa a ser interno y se auto-resuelve según `Environment` (integración/producción).
- Documentación actualizada para arquitectura v1.1.0.

### Compatibility

- API legacy (`Start`, `Finish`, `Remove`, `Authorize`, `Status`, `Refund`, `Capture`) se mantiene compatible.
