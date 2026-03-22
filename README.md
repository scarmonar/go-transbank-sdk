# go-transbank-sdk

SDK en Go para **Transbank Oneclick Mall v1.2**.

Fuente oficial única de contrato API:
- https://www.transbankdevelopers.cl/referencia/oneclick

## Estado del proyecto

Este SDK está orientado a paridad con el contrato REST oficial de Oneclick Mall.

Operaciones soportadas:

- `POST /inscriptions` (iniciar inscripción)
- `PUT /inscriptions/{token}` (finalizar inscripción)
- `DELETE /inscriptions` (eliminar inscripción)
- `POST /transactions` (autorizar transacción)
- `GET /transactions/{buyOrder}` (consultar estado)
- `POST /transactions/{buyOrder}/refunds` (reversar/anular)
- `PUT /transactions/capture` (captura diferida)

## Requisitos

- Go `1.26.1` (según `go.mod`)
- Código de comercio y API Key Secret de Transbank

## Instalación

```bash
go get github.com/scarmonar/go-transbank-sdk
```

## Inicialización del servicio

```go
svc, err := oneclick.NewOneclickService(
	"597055555541", // Código comercio mall (padre)
	"TU_API_KEY_SECRET",
	"https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2", // Integración
	nil, // http.Client opcional
)
if err != nil {
	panic(err)
}
```

## API pública

Métodos disponibles en `oneclick.OneclickService`:

- `Start(ctx, username, email, responseURL)`
- `Finish(ctx, token)`
- `Remove(ctx, tbkUser, username)`
- `Authorize(ctx, username, tbkUser, buyOrder, details)`
- `Status(ctx, buyOrder)`
- `Refund(ctx, buyOrder, commerceCode, detailBuyOrder, amount)`
- `Capture(ctx, commerceCode, buyOrder, authorizationCode, captureAmount)`

## Flujo recomendado

### 1) Iniciar inscripción

```go
resp, err := svc.Start(ctx,
	"usuario_123",
	"user@example.com",
	"https://mi-comercio.cl/oneclick/retorno",
)
if err != nil {
	panic(err)
}

// Redireccionar navegador del usuario a resp.URLWebpay
// enviando TBK_TOKEN=resp.Token
```

### 2) Finalizar inscripción

```go
finishResp, err := svc.Finish(ctx, tbkToken)
if err != nil {
	panic(err)
}

fmt.Println("tbk_user:", finishResp.TbkUser)
```

### 3) Autorizar cobro

```go
authResp, err := svc.Authorize(ctx,
	"usuario_123",
	finishResp.TbkUser,
	"parent-order-1",
	[]oneclick.TransactionDetail{
		{
			CommerceCode:       "597055555542", // comercio hijo
			BuyOrder:           "child-order-1",
			Amount:             50000,
			InstallmentsNumber: 0,
		},
	},
)
if err != nil {
	panic(err)
}

for _, d := range authResp.Details {
	fmt.Println(d.Status, d.ResponseCode, d.AuthorizationCode)
}
```

### 4) Consultar estado

```go
statusResp, err := svc.Status(ctx, "parent-order-1")
if err != nil {
	panic(err)
}

fmt.Println("detalle(s):", len(statusResp.Details))
```

### 5) Reversar o anular

```go
refundResp, err := svc.Refund(ctx,
	"parent-order-1",
	"597055555542",
	"child-order-1",
	50000,
)
if err != nil {
	panic(err)
}

fmt.Println("tipo:", refundResp.Type) // REVERSED o NULLIFIED
```

### 6) Captura diferida

```go
captureResp, err := svc.Capture(ctx,
	"597055555542", // comercio hijo
	"child-order-1",
	"1213",         // authorization_code
	50000,
)
if err != nil {
	panic(err)
}

fmt.Println("captured_amount:", captureResp.CapturedAmount)
```

## Estructuras principales

- `InscriptionRequest` / `InscriptionResponse`
- `InscriptionConfirmResponse`
- `TransactionDetail`
- `AuthorizeTransactionRequest` / `AuthorizeTransactionResponse`
- `TransactionResponseDetail`
- `RefundRequest` / `RefundResponse`
- `CaptureRequest` / `CaptureResponse`

Tipos de pago (`payment_type_code`) disponibles como constantes:

- `PaymentTypeDebit` (`VD`)
- `PaymentTypePrepaid` (`VP`)
- `PaymentTypeNormalSale` (`VN`)
- `PaymentTypeInstallments` (`VC`)
- `PaymentType3InstallmentsNoInt` (`SI`)
- `PaymentType2InstallmentsNoInt` (`S2`)
- `PaymentTypeNInstallmentsNoInt` (`NC`)

Estados de transacción disponibles como constantes:

- `INITIALIZED`, `AUTHORIZED`, `REVERSED`, `FAILED`, `NULLIFIED`, `PARTIALLY_NULLIFIED`, `CAPTURED`

## Reglas de validación

Validaciones implementadas en el SDK (alineadas al contrato oficial):

- `username`: obligatorio, máximo 40 caracteres
- `email`: obligatorio, formato básico con `@`, máximo 100 caracteres
- `response_url`: obligatoria, URL absoluta, máximo 255 caracteres
- `tbk_user`: obligatorio, máximo 40 caracteres
- `buy_order`: obligatorio, máximo 26 caracteres, con caracteres permitidos por Transbank
- `commerce_code`: obligatorio, máximo 12 caracteres
- `authorization_code`: obligatorio, máximo 6 caracteres
- `amount` y `capture_amount`: deben ser `> 0`
- `installments_number`:
  - si es negativo, se normaliza a `0`
  - si es mayor a `99`, retorna error

## Manejo de errores

Errores de API se exponen como `*oneclick.TransbankError`.

```go
resp, err := svc.Finish(ctx, token)
if err != nil {
	var tbkErr *oneclick.TransbankError
	if errors.As(err, &tbkErr) {
		fmt.Println("codigo:", tbkErr.Code)
		fmt.Println("mensaje:", tbkErr.Message)
		fmt.Println("detalle:", tbkErr.Details)
	}
}
```

El parser considera, entre otros, payloads como:

```json
{"error_message":"token is required"}
```

## Ambientes

Integración:

- `https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2`

Producción:

- `https://webpay3g.transbank.cl/rswebpaytransaction/api/oneclick/v1.2`

## Credenciales de integración (documentación Transbank)

- Comercio mall (padre): `597055555541`
- Comercios hijo: `597055555542`, `597055555543`
- API Key Secret: `579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C`

## Ejemplos incluidos

- `examples/simple_inscription/main.go`
- `examples/simple_charge/main.go`

## Pruebas

```bash
go test ./...
```

## Recomendaciones de seguridad

- No hardcodear credenciales en código fuente.
- Usar variables de entorno o un secret manager.
- Rotar credenciales si se exponen accidentalmente.

## Referencias

- Referencia API Oneclick: https://www.transbankdevelopers.cl/referencia/oneclick
- Documentación funcional: https://transbankdevelopers.cl/documentacion/oneclick
