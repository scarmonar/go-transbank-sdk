# Índice de Documentación

Este repositorio mantiene su documentación principal en los siguientes archivos.

## Documentación principal

- `README.md`
  - Guía general del SDK Oneclick Mall v1.2.
  - API pública, ejemplos de uso y recomendaciones de seguridad.
  - Inicialización simplificada:
    - `NewOneclickService()` para integración
    - `NewOneclickServiceFor(...)` para producción/avanzado

- `INTEGRATION_TESTS.md`
  - Guía actualizada de pruebas de integración reales.
  - Comandos de ejecución (`TRANSBANK_RUN_INTEGRATION_TESTS=1 ...`).
  - Variables de entorno y fixtures opcionales.

## Código fuente relevante

- `oneclick/service.go`
  - Implementación del cliente y operaciones del API.

- `oneclick/types.go`
  - Tipos de request/response.

- `oneclick/errors.go`
  - Errores tipados del SDK.

- `oneclick/service_test.go`
  - Tests unitarios de contrato y validaciones.

- `oneclick/integration_test.go`
  - Tests reales contra ambiente de integración.

## Ejemplos

- `examples/simple_inscription/main.go`
- `examples/simple_charge/main.go`

## Referencias oficiales

- API Oneclick: https://www.transbankdevelopers.cl/referencia/oneclick
- Ambientes: https://www.transbankdevelopers.cl/documentacion/como_empezar#ambientes
