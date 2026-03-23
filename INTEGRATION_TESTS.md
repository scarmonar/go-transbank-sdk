# Pruebas de Integración Reales - Transbank Oneclick

Esta guía describe la suite de pruebas reales contra el ambiente de integración de Transbank para Oneclick Mall v1.2.

Fuente oficial de contrato:
- https://www.transbankdevelopers.cl/referencia/oneclick

## Estado actual

Archivo de tests:
- `oneclick/integration_test.go`

Cobertura actual:
- 11 tests de integración real
- 7 operaciones del API cubiertas (`Start`, `Finish`, `Remove`, `Authorize`, `Status`, `Refund`, `Capture`)
- smoke tests no destructivos + escenarios mutantes con fixtures

Notas de versión SDK:
- Desde `v1.1.0`, el SDK expone `Client` (raw) y `FlowService` (alto nivel).
- Desde `v1.2.0`, `FlowService` agrega `AuthorizeCharge`/`ReverseCharge` y `FlowConfirmResponse` con contexto expandido.
- Desde `v1.2.0`, existe `ClassifyError(err)` para clasificación estable de errores en integradores.
- La suite de integración real valida el contrato Transbank usando el cliente raw (vía API compatible `OneclickService`).
- La capa `FlowService` se valida principalmente con unit tests (`oneclick/flow_test.go`).

## Requisitos

- Go `1.16+`
- Conectividad a `webpay3gint.transbank.cl`
- `TRANSBANK_RUN_INTEGRATION_TESTS=1`

Credenciales/defaults usados por la suite:
- Comercio mall: `597055555541`
- API Secret: `579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C`
- Base URL: `https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2`
- Response URL: `https://example.com/oneclick/return`

## Ejecución

### 1) Smoke tests reales (recomendado)

```bash
TRANSBANK_RUN_INTEGRATION_TESTS=1 go test ./oneclick -run TestIntegrationOneclick -v -count=1
```

### 2) Escenarios mutantes/happy-path (con fixtures)

```bash
TRANSBANK_RUN_INTEGRATION_TESTS=1 \
TRANSBANK_TEST_RUN_MUTATING=1 \
go test ./oneclick -run 'HappyPath|WithFixture|RemoveWithFixture' -v -count=1
```

### 3) Solo unit tests del SDK (sin red)

```bash
go test ./oneclick -run 'TestFlow|TestBuild|TestClientHooks|TestClassifyError|TestHooksKeepSameRequestIDAcrossRetries|TestNewOneclickService|TestStartRequestShape' -v
```

## Matriz de tests

### Smoke / sin fixtures

- `TestIntegrationOneclickStart`
- `TestIntegrationOneclickFinishWithoutCustomerInteraction`
- `TestIntegrationOneclickRemoveUnknown`
- `TestIntegrationOneclickAuthorizeUnknownUser`
- `TestIntegrationOneclickStatusUnknownBuyOrder`
- `TestIntegrationOneclickRefundUnknownBuyOrder`
- `TestIntegrationOneclickCaptureUnknown`

### Con fixtures (opcionales)

- `TestIntegrationOneclickFinishWithFixture`
- `TestIntegrationOneclickRemoveWithFixture`
- `TestIntegrationOneclickAuthorizeStatusRefundHappyPath`
- `TestIntegrationOneclickCaptureWithFixture`

## Variables de entorno

### Obligatorias

- `TRANSBANK_RUN_INTEGRATION_TESTS=1`

### Para mutaciones

- `TRANSBANK_TEST_RUN_MUTATING=1`

### Overrides generales (opcionales)

- `TRANSBANK_BASE_URL`
- `TRANSBANK_COMMERCE_CODE`
- `TRANSBANK_API_SECRET`
- `TRANSBANK_RESPONSE_URL`
- `TRANSBANK_TEST_EMAIL_DOMAIN`
- `TRANSBANK_TEST_CHILD_COMMERCE_CODE`
- `TRANSBANK_TEST_AMOUNT`

### Fixtures por escenario

Finish exitoso:
- `TRANSBANK_TEST_FINISH_TOKEN`

Remove exitoso:
- `TRANSBANK_TEST_REMOVE_USERNAME`
- `TRANSBANK_TEST_REMOVE_TBK_USER`

Authorize + Status + Refund happy-path:
- `TRANSBANK_TEST_USERNAME`
- `TRANSBANK_TEST_TBK_USER`

Capture exitosa:
- `TRANSBANK_TEST_CAPTURE_COMMERCE_CODE`
- `TRANSBANK_TEST_CAPTURE_BUY_ORDER`
- `TRANSBANK_TEST_CAPTURE_AUTH_CODE`
- `TRANSBANK_TEST_CAPTURE_AMOUNT`

## Comportamiento esperado

- Si falta `TRANSBANK_RUN_INTEGRATION_TESTS=1`, toda la suite se marca `SKIP`.
- Si faltan fixtures de un escenario opcional, ese test se marca `SKIP`.
- Los tests generan `buy_order`/usuarios dinámicos para reducir colisiones.
- En escenarios `Unknown`, un error HTTP/API puede ser el resultado esperado del test.

## Troubleshooting

- Error de red/DNS: valida conectividad hacia `webpay3gint.transbank.cl`.
- Timeouts: sube timeout de shell o reintenta por estabilidad de red.
- Falla en mutantes: revisa fixtures, vigencia de token y datos de inscripción.
- Error por credenciales: revisa `TRANSBANK_COMMERCE_CODE`/`TRANSBANK_API_SECRET` y ambiente.

## Referencias

- API Oneclick: https://www.transbankdevelopers.cl/referencia/oneclick
- Ambientes: https://www.transbankdevelopers.cl/documentacion/como_empezar#ambientes
- Guía rápida: `INTEGRATION_TESTS_QUICK_REF.md`
- Guía completa: `INTEGRATION_TESTS_COMPLETE.md`
- README del SDK: `README.md`
