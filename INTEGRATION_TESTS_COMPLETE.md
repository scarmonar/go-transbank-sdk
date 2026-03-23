# Pruebas de Integración Reales - Guía Completa

Esta guía es la versión extendida para operar, depurar y mantener la suite de integración real del SDK.

## 1. Objetivo

Validar extremo a extremo el contrato de Oneclick Mall v1.2 contra el ambiente de integración de Transbank, reduciendo regresiones en:

- endpoints
- validaciones de payload
- parsing de respuestas y errores
- comportamiento esperado ante datos inexistentes

## 2. Scope de pruebas

Archivo:
- `oneclick/integration_test.go`

Escenarios cubiertos:
- `Start`
- `Finish`
- `Remove`
- `Authorize`
- `Status`
- `Refund`
- `Capture`

Total actual:
- 11 tests

Distribución:
- 7 smoke tests no destructivos
- 4 tests con fixtures/happy-path (mutantes)

## 3. Requisitos previos

- Go `1.16+`
- Acceso de red saliente a `webpay3gint.transbank.cl`
- Variables de entorno para habilitación (`TRANSBANK_RUN_INTEGRATION_TESTS=1`)

Opcional:
- Fixtures reales para escenarios exitosos controlados

## 4. Configuración base

Variables y defaults usados por el test harness:

- `TRANSBANK_BASE_URL` default `https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2`
- `TRANSBANK_COMMERCE_CODE` default `597055555541`
- `TRANSBANK_API_SECRET` default credencial pública integración
- `TRANSBANK_RESPONSE_URL` default `https://example.com/oneclick/return`
- `TRANSBANK_TEST_EMAIL_DOMAIN` default `example.com`
- `TRANSBANK_TEST_CHILD_COMMERCE_CODE` default `597055555542`
- `TRANSBANK_TEST_AMOUNT` default `1000`

## 5. Ejecución recomendada

### Paso A: smoke suite

```bash
TRANSBANK_RUN_INTEGRATION_TESTS=1 go test ./oneclick -run TestIntegrationOneclick -v -count=1
```

Criterio de éxito:
- smoke tests pasan
- tests con fixtures pueden quedar `SKIP` si no configuras variables

### Paso B: happy-path con fixtures

```bash
TRANSBANK_RUN_INTEGRATION_TESTS=1 \
TRANSBANK_TEST_RUN_MUTATING=1 \
go test ./oneclick -run 'HappyPath|WithFixture|RemoveWithFixture' -v -count=1
```

Criterio de éxito:
- solo ejecutar cuando fixtures son válidos y vigentes
- revisar logs de cada test para verificar el flujo real

## 6. Fixtures detallados

Finish exitoso:
- `TRANSBANK_TEST_FINISH_TOKEN`

Remove exitoso:
- `TRANSBANK_TEST_REMOVE_USERNAME`
- `TRANSBANK_TEST_REMOVE_TBK_USER`

Authorize + Status + Refund:
- `TRANSBANK_TEST_USERNAME`
- `TRANSBANK_TEST_TBK_USER`

Capture exitosa:
- `TRANSBANK_TEST_CAPTURE_COMMERCE_CODE`
- `TRANSBANK_TEST_CAPTURE_BUY_ORDER`
- `TRANSBANK_TEST_CAPTURE_AUTH_CODE`
- `TRANSBANK_TEST_CAPTURE_AMOUNT`

## 7. Convenciones y comportamiento

- Si `TRANSBANK_RUN_INTEGRATION_TESTS` no es `1`, toda la suite se salta (`SKIP`).
- Si faltan fixtures, el escenario dependiente se salta (`SKIP`).
- `buy_order` y `username` se generan dinámicamente para evitar colisiones.
- Casos `Unknown` esperan rechazo/error como resultado válido.

## 8. Relación con v1.2.0

Desde v1.2.0:

- El SDK tiene `Client` (raw) + `FlowService` (alto nivel).
- `FlowService` agrega `AuthorizeCharge`/`ReverseCharge` para reducir lógica manual del integrador.
- `FlowConfirmResponse` incluye contexto expandido (`BusinessID`, `SubscriptionID`, `Context`).
- Se incorpora `ClassifyError(err)` para clasificación estable de errores.
- Estas pruebas integran el contrato Transbank con el cliente raw.
- `FlowService` se prueba mayormente con unit tests (`oneclick/flow_test.go`) y puede sumarse un set de integración dedicado en el futuro.

## 9. Troubleshooting avanzado

Problema: DNS / conexión
- validar resolución y reachability de `webpay3gint.transbank.cl`
- repetir ejecución fuera de horarios de alta latencia

Problema: timeout intermitente
- reintentar con `-count=1`
- revisar conectividad estable antes de diagnosticar SDK

Problema: fixture inválido
- regenerar token/inscripción
- verificar que `username` y `tbk_user` pertenezcan a la misma inscripción

Problema: credenciales
- confirmar `commerce code` y `api secret`
- confirmar que el endpoint sea de integración

## 10. Checklist para CI/CD

- Ejecutar unit tests siempre: `go test ./...`
- Ejecutar smoke integration en job opcional/manual
- Ejecutar mutantes solo con fixtures controlados
- Publicar logs de pruebas para soporte post-falla

## 11. Referencias

- API Oneclick: https://www.transbankdevelopers.cl/referencia/oneclick
- Ambientes: https://www.transbankdevelopers.cl/documentacion/como_empezar#ambientes
- Guía principal: `INTEGRATION_TESTS.md`
- Quick ref: `INTEGRATION_TESTS_QUICK_REF.md`
