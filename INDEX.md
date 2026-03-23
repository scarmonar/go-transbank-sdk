# Índice de Documentación

## Documentación principal

- `README.md`
  - Guía general del SDK Oneclick Mall v1.2.1.
  - Arquitectura v1.2.x: `Client` (raw) + `FlowService` (alto nivel).
  - Nuevos flujos de cargo/reversa de alto nivel, clasificación estable de errores y helper de rechazo por `response_code`.
  - Configuración, opciones, errores tipados, observabilidad y ejemplos.

- `INTEGRATION_TESTS.md`
  - Guía de pruebas de integración reales.
  - Variables de entorno y fixtures opcionales.

- `INTEGRATION_TESTS_QUICK_REF.md`
  - Cheatsheet operativo de comandos y variables para ejecución rápida.

- `INTEGRATION_TESTS_COMPLETE.md`
  - Guía extendida de operación, troubleshooting y checklist para CI/CD.

## Código fuente relevante

- `oneclick/service.go`
  - Cliente raw `Client` y wrappers legacy (`OneclickService`).
  - Resiliencia: retries + circuit breaker + hooks + métricas.

- `oneclick/config.go`
  - `Config`, defaults, autodetección de ambiente y options pattern.

- `oneclick/flow.go`
  - `FlowService`, `BuildReturnURL`, redirect instruction e idempotencia.
  - `FlowConfirmResponse` con contexto expandido.
  - `AuthorizeCharge` / `ReverseCharge`.

- `oneclick/state_store.go`
  - `StateStore` + implementaciones in-memory/Postgres/Redis.

- `oneclick/errors.go`
  - `SDKError` tipado + `TransbankError`.
  - Helper `ClassifyError(err)`.

- `oneclick/response_codes.go`
  - `ClassifyResponseCode(code)` para traducir rechazos de Transbank a mensajes de negocio.

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
