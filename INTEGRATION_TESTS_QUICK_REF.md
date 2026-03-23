# INTEGRATION TESTS - Quick Reference

## Comandos

Smoke tests reales:

```bash
TRANSBANK_RUN_INTEGRATION_TESTS=1 go test ./oneclick -run TestIntegrationOneclick -v -count=1
```

Mutantes/happy-path (con fixtures):

```bash
TRANSBANK_RUN_INTEGRATION_TESTS=1 \
TRANSBANK_TEST_RUN_MUTATING=1 \
go test ./oneclick -run 'HappyPath|WithFixture|RemoveWithFixture' -v -count=1
```

Unit tests del SDK (sin red):

```bash
go test ./oneclick -run 'TestFlow|TestBuild|TestClientHooks|TestClassifyError|TestHooksKeepSameRequestIDAcrossRetries' -v
```

## Variables mínimas

Siempre:

```bash
export TRANSBANK_RUN_INTEGRATION_TESTS=1
```

Para mutaciones:

```bash
export TRANSBANK_TEST_RUN_MUTATING=1
```

## Fixtures opcionales

```bash
# Finish
export TRANSBANK_TEST_FINISH_TOKEN="..."

# Remove
export TRANSBANK_TEST_REMOVE_USERNAME="..."
export TRANSBANK_TEST_REMOVE_TBK_USER="..."

# Authorize/Status/Refund
export TRANSBANK_TEST_USERNAME="..."
export TRANSBANK_TEST_TBK_USER="..."

# Capture
export TRANSBANK_TEST_CAPTURE_COMMERCE_CODE="..."
export TRANSBANK_TEST_CAPTURE_BUY_ORDER="..."
export TRANSBANK_TEST_CAPTURE_AUTH_CODE="..."
export TRANSBANK_TEST_CAPTURE_AMOUNT="1000"
```

## Defaults útiles

- `TRANSBANK_BASE_URL=https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2`
- `TRANSBANK_COMMERCE_CODE=597055555541`
- `TRANSBANK_API_SECRET=579B...36B1C`
- `TRANSBANK_RESPONSE_URL=https://example.com/oneclick/return`

## Qué valida cada bloque

- Smoke: contrato base y respuestas esperadas en casos normales/unknown.
- Fixtures: happy paths reales de `Finish`, `Remove`, `Authorize+Status+Refund`, `Capture`.

## Si algo falla rápido

- Verifica red a `webpay3gint.transbank.cl`.
- Verifica credenciales/ambiente.
- Verifica vigencia y consistencia de fixtures.
- Reintenta: hay variabilidad temporal del ambiente de integración.
