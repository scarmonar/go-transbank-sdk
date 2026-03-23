# Índice de Documentación

## Documentación principal

- `README.md`
  - Guía general del SDK Oneclick Mall v1.2.
  - Arquitectura v1.1.0: `Client` (raw) + `FlowService` (alto nivel).
  - Configuración, opciones, errores tipados, observabilidad y ejemplos.

- `INTEGRATION_TESTS.md`
  - Guía de pruebas de integración reales.
  - Variables de entorno y fixtures opcionales.

## Código fuente relevante

- `oneclick/service.go`
  - Cliente raw `Client` y wrappers legacy (`OneclickService`).
  - Resiliencia: retries + circuit breaker + hooks + métricas.

- `oneclick/config.go`
  - `Config`, defaults, autodetección de ambiente y options pattern.

- `oneclick/flow.go`
  - `FlowService`, `BuildReturnURL`, redirect instruction e idempotencia.

- `oneclick/state_store.go`
  - `StateStore` + implementaciones in-memory/Postgres/Redis.

- `oneclick/errors.go`
  - `SDKError` tipado + `TransbankError`.

- `oneclick/types.go`
  - Tipos de request/response.

- `oneclick/service_test.go`
  - Tests unitarios del cliente raw.

- `oneclick/flow_test.go`
  - Tests de flujo alto nivel, idempotencia y hooks.

- `oneclick/integration_test.go`
  - Tests reales contra ambiente de integración.

## Ejemplos

- `examples/simple_inscription/main.go`
- `examples/simple_charge/main.go`

## Referencias oficiales

- API Oneclick: https://www.transbankdevelopers.cl/referencia/oneclick
- Ambientes: https://www.transbankdevelopers.cl/documentacion/como_empezar#ambientes
