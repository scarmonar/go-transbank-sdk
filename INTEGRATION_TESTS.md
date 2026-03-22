# Pruebas de Integración Reales - Transbank Oneclick

Esta guía describe la suite de pruebas reales contra el ambiente de integración de Transbank para Oneclick Mall v1.2.

Fuente oficial de contrato:
- https://www.transbankdevelopers.cl/referencia/oneclick

## Estado actual de la suite

Archivo de tests:
- `oneclick/integration_test.go`

Cobertura actual:
- 11 tests de integración real
- 7 operaciones del API cubiertas (`Start`, `Finish`, `Remove`, `Authorize`, `Status`, `Refund`, `Capture`)
- smoke tests no destructivos + escenarios mutantes con fixtures

## Requisitos

- Go `1.26.1+`
- Conectividad a `webpay3gint.transbank.cl`
- Variable `TRANSBANK_RUN_INTEGRATION_TESTS=1` para habilitar ejecución

Credenciales por defecto usadas por la suite:
- Comercio mall: `597055555541`
- API Secret: `579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C`
- Base URL: `https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2`

## Cómo ejecutar

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

## Matriz de tests

### Smoke / comportamiento esperado sin fixtures

- `TestIntegrationOneclickStart`: inicia inscripción real
- `TestIntegrationOneclickFinishWithoutCustomerInteraction`: valida error esperado al confirmar sin interacción de usuario
- `TestIntegrationOneclickRemoveUnknown`: intenta eliminar inscripción inexistente
- `TestIntegrationOneclickAuthorizeUnknownUser`: intenta autorizar con usuario/tbk_user inexistente
- `TestIntegrationOneclickStatusUnknownBuyOrder`: consulta estado para `buy_order` inexistente
- `TestIntegrationOneclickRefundUnknownBuyOrder`: reversa/anula para transacción inexistente
- `TestIntegrationOneclickCaptureUnknown`: captura para transacción inexistente

### Con fixtures (opcionales)

- `TestIntegrationOneclickFinishWithFixture`
- `TestIntegrationOneclickRemoveWithFixture`
- `TestIntegrationOneclickAuthorizeStatusRefundHappyPath`
- `TestIntegrationOneclickCaptureWithFixture`

## Variables de entorno

### Habilitación

- `TRANSBANK_RUN_INTEGRATION_TESTS=1` (obligatoria)
- `TRANSBANK_TEST_RUN_MUTATING=1` (obligatoria para tests mutantes)

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

## Troubleshooting rápido

- Error de red/DNS: valida conectividad hacia `webpay3gint.transbank.cl`.
- Errores 4xx/5xx esperados en tests `Unknown`: el test puede pasar con error de API porque ese es el comportamiento validado.
- Si un test mutante falla por datos: revisa fixtures y que sean recientes/válidos.

## Referencias

- API Oneclick: https://www.transbankdevelopers.cl/referencia/oneclick
- Ambientes: https://www.transbankdevelopers.cl/documentacion/como_empezar#ambientes
- README del SDK: `README.md`
