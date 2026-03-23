# go-transbank-sdk

SDK en Go para **Transbank Oneclick Mall v1.2** con dos niveles de API:

- `oneclick.Client`: cliente raw (contrato Transbank 1:1).
- `oneclick.FlowService`: capa opinionated para flujo completo de inscripción.

Fuente oficial del contrato API:
- https://www.transbankdevelopers.cl/referencia/oneclick

## Novedades v1.1.0

- Arquitectura en dos capas: `Client` + `FlowService`.
- `Config` centralizado con defaults sanos y `Validate()` fail-fast.
- Options pattern: `WithHTTPClient`, `WithLogger`, `WithRetryPolicy`, `WithClock`, `WithBaseURL`.
- Resiliencia integrada: timeout, retries y circuit breaker.
- Errores tipados SDK: `ErrValidation`, `ErrTransport`, `ErrGateway`, `ErrTokenNotFound`, `ErrFlowState`.
- Observabilidad: hooks `BeforeRequest`, `AfterRequest`, `OnError` + métricas internas.
- Estado de flujo por token (`StateStore`) con implementación in-memory y adapters `Postgres` / `Redis`.
- Idempotencia opcional en `FlowService.StartInscription` y `FlowService.ConfirmInscription`.

## Requisitos

- Go `1.16+`
- Comercio y API Key Secret de Transbank (en producción)

## Instalación

```bash
go get github.com/scarmonar/go-transbank-sdk
```

## Importación

```go
import "github.com/scarmonar/go-transbank-sdk/oneclick"
```

## Quick Start (Raw Client)

```go
client, err := oneclick.NewClient()
if err != nil {
	panic(err)
}

resp, err := client.Start(context.Background(),
	"usuario_123",
	"user@example.com",
	"https://mi-comercio.cl/oneclick/retorno",
)
if err != nil {
	panic(err)
}

fmt.Println(resp.Token, resp.URLWebpay)
```

### Producción explícita

```go
client, err := oneclick.NewOneclickServiceFor(
	oneclick.EnvironmentProduction,
	"TU_COMMERCE_CODE",
	"TU_API_KEY_SECRET",
	nil,
)
if err != nil {
	panic(err)
}
```

`NewOneclickService` y `NewOneclickServiceFor` se mantienen por compatibilidad; internamente usan `Client`.

## Config + Options

```go
cfg := oneclick.DefaultConfig()
cfg.Environment = oneclick.EnvironmentProduction
cfg.CommerceCode = "TU_COMMERCE_CODE"
cfg.APISecret = "TU_SECRET"

client, err := oneclick.NewClientWithConfig(
	cfg,
	oneclick.WithBaseURL("https://webpay3g.transbank.cl/rswebpaytransaction/api/oneclick/v1.2"),
)
if err != nil {
	panic(err)
}
_ = client
```

Ambiente autodetectado:
- Si usas credenciales públicas de integración o vacías en integración, usa `integration`.
- Si pasas credenciales custom sin ambiente explícito, asume `production`.
- Si `BaseURL` apunta a `webpay3gint`, detecta `integration`.

## API Raw (`oneclick.Client`)

- `StartInscription(ctx, InscriptionRequest)`
- `ConfirmInscription(ctx, token)`
- `DeleteInscription(ctx, DeleteInscriptionRequest)`
- `AuthorizeTransaction(ctx, AuthorizeTransactionRequest)`
- `GetTransactionStatus(ctx, buyOrder)`
- `RefundTransaction(ctx, buyOrder, RefundRequest)`
- `CaptureTransaction(ctx, CaptureRequest)`

Wrappers legacy (compat):
- `Start`, `Finish`, `Remove`, `Authorize`, `Status`, `Refund`, `Capture`

## FlowService (Capa Opinionated)

`FlowService` encapsula:
- construcción segura de `response_url`
- instrucción de redirect a Webpay (`POST + TBK_TOKEN`)
- estado de flujo por token
- idempotencia opcional en `Start` y `Confirm`

```go
client, _ := oneclick.NewClient()
store := oneclick.NewInMemoryStateStore()
flow, _ := oneclick.NewFlowService(client, store)

startResp, err := flow.StartInscription(context.Background(), oneclick.FlowStartRequest{
	Username:       "usuario_123",
	Email:          "user@example.com",
	BaseURL:        "https://mi-comercio.cl",
	ReturnPath:     "/api/billing/confirm",
	ReturnMode:     oneclick.ReturnURLModeStrictHTTPS,
	BusinessID:     "biz-1",
	SubscriptionID: "sub-1",
	IdempotencyKey: "start-sub-1-v1",
})
if err != nil {
	panic(err)
}

// Ejecutar redirect en frontend/backend con esta instrucción
fmt.Println(startResp.Redirect.Method)             // POST
fmt.Println(startResp.Redirect.URLWebpay)          // url_webpay
fmt.Println(startResp.Redirect.FormFields["TBK_TOKEN"]) // token

confirmResp, err := flow.ConfirmInscription(context.Background(), oneclick.FlowConfirmRequest{
	Token:          startResp.Token,
	IdempotencyKey: "confirm-sub-1-v1",
})
if err != nil {
	panic(err)
}

fmt.Println(confirmResp.State.Status)        // confirmed
fmt.Println(confirmResp.Confirmation.TbkUser)
```

## Return URL Builder

```go
url, err := flow.BuildReturnURL(
	"https://mi-comercio.cl",
	"/api/billing/confirm",
	oneclick.ReturnURLModeStrictHTTPS,
)
```

Reglas:
- normaliza host/scheme
- elimina query/fragment
- valida formato y longitud
- modo estricto `https`
- modo local opcional `http` solo para localhost/loopback

## StateStore

Interfaz:
- `GetByToken(ctx, token)`
- `SavePending(ctx, state)`
- `MarkConfirmed(ctx, token, confirmation)`

Implementaciones incluidas:
- `NewInMemoryStateStore()`
- `NewPostgresStateStore(db, tableName)`
- `NewRedisStateStore(client, prefix, ttl)`

## Errores Tipados SDK

Tipos de error canónicos:
- `ErrValidation`
- `ErrTransport`
- `ErrGateway`
- `ErrTokenNotFound`
- `ErrFlowState`

Todos exponen:
- `Code()`
- `Retryable()`
- `UserSafeMessage()`

Además, errores de API Transbank siguen disponibles como `*oneclick.TransbankError`.

```go
if err != nil {
	var sdkErr *oneclick.SDKError
	if errors.As(err, &sdkErr) {
		fmt.Println("code:", sdkErr.Code())
		fmt.Println("retryable:", sdkErr.Retryable())
		fmt.Println("safe:", sdkErr.UserSafeMessage())
	}

	var tbkErr *oneclick.TransbankError
	if errors.As(err, &tbkErr) {
		fmt.Println("tbk code:", tbkErr.Code)
	}
}
```

## Observabilidad y Métricas

Hooks configurables (`WithHooks`):
- `BeforeRequest`
- `AfterRequest`
- `OnError`

Campos estándar disponibles en eventos:
- `request_id`
- `token_length`
- `response_code`

Métricas internas:
- latencia promedio
- tasa de errores
- fallos de confirmación

```go
snapshot := client.MetricsSnapshot()
fmt.Println(snapshot.RequestsTotal, snapshot.ErrorRate, snapshot.ConfirmationFailures)
```

## Pruebas

```bash
go test ./...
```

Suite de integración real:

```bash
TRANSBANK_RUN_INTEGRATION_TESTS=1 go test ./oneclick -run TestIntegrationOneclick -v -count=1
```

## Seguridad

- Las credenciales embebidas son solo para integración.
- En producción, inyectar secretos externos.
- No hardcodear credenciales productivas.

## Referencias

- API Oneclick: https://www.transbankdevelopers.cl/referencia/oneclick
- Documentación funcional: https://transbankdevelopers.cl/documentacion/oneclick
