package oneclick

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	defaultIntegrationChildCode   = "597055555542"
	defaultIntegrationResponseURL = "https://example.com/oneclick/return"
)

type integrationConfig struct {
	BaseURL      string
	CommerceCode string
	APISecret    string
	ResponseURL  string
	EmailDomain  string
	ChildCode    string
	Amount       int

	RunMutating bool

	Username string
	TbkUser  string

	FinishToken string

	RemoveUsername string
	RemoveTbkUser  string

	CaptureCode   string
	CaptureOrder  string
	CaptureAuth   string
	CaptureAmount int
}

func TestIntegrationOneclickStart(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	svc := newIntegrationService(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	username := randomUsername("it-start")
	email := fmt.Sprintf("%s@%s", username, cfg.EmailDomain)

	resp, err := svc.Start(ctx, username, email, cfg.ResponseURL)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if strings.TrimSpace(resp.Token) == "" {
		t.Fatal("start returned empty token")
	}
	if strings.TrimSpace(resp.URLWebpay) == "" {
		t.Fatal("start returned empty url_webpay")
	}
	if !strings.Contains(strings.ToLower(resp.URLWebpay), "webpay") {
		t.Fatalf("unexpected url_webpay: %s", resp.URLWebpay)
	}
}

func TestIntegrationOneclickFinishWithoutCustomerInteraction(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	svc := newIntegrationService(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	username := randomUsername("it-finish")
	email := fmt.Sprintf("%s@%s", username, cfg.EmailDomain)
	startResp, err := svc.Start(ctx, username, email, cfg.ResponseURL)
	if err != nil {
		t.Fatalf("start precondition failed: %v", err)
	}

	_, err = svc.Finish(ctx, startResp.Token)
	if err == nil {
		t.Fatal("expected finish error without customer completing webpay inscription")
	}
	t.Logf("finish without customer flow returned expected error: %v", err)
}

func TestIntegrationOneclickFinishWithFixture(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	if cfg.FinishToken == "" {
		t.Skip("set TRANSBANK_TEST_FINISH_TOKEN to run finish success test")
	}

	svc := newIntegrationService(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	resp, err := svc.Finish(ctx, cfg.FinishToken)
	if err != nil {
		t.Fatalf("finish with fixture token failed: %v", err)
	}
	if resp.ResponseCode != 0 {
		t.Fatalf("unexpected response_code: %d", resp.ResponseCode)
	}
	if strings.TrimSpace(resp.TbkUser) == "" {
		t.Fatal("finish returned empty tbk_user")
	}
}

func TestIntegrationOneclickRemoveUnknown(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	svc := newIntegrationService(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	err := svc.Remove(ctx, randomTbkUser(), randomUsername("it-rm"))
	if err == nil {
		t.Fatal("expected remove error for unknown inscription")
	}
	t.Logf("remove unknown returned expected error: %v", err)
}

func TestIntegrationOneclickRemoveWithFixture(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	if !cfg.RunMutating {
		t.Skip("set TRANSBANK_TEST_RUN_MUTATING=1 to run mutating integration tests")
	}
	if cfg.RemoveTbkUser == "" || cfg.RemoveUsername == "" {
		t.Skip("set TRANSBANK_TEST_REMOVE_TBK_USER and TRANSBANK_TEST_REMOVE_USERNAME to run remove fixture test")
	}

	svc := newIntegrationService(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	err := svc.Remove(ctx, cfg.RemoveTbkUser, cfg.RemoveUsername)
	if err != nil {
		t.Fatalf("remove with fixture failed: %v", err)
	}
}

func TestIntegrationOneclickAuthorizeUnknownUser(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	svc := newIntegrationService(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	resp, err := svc.Authorize(ctx,
		randomUsername("it-auth"),
		randomTbkUser(),
		randomBuyOrder("parent"),
		[]TransactionDetail{
			{
				CommerceCode:       cfg.ChildCode,
				BuyOrder:           randomBuyOrder("child"),
				Amount:             cfg.Amount,
				InstallmentsNumber: 0,
			},
		},
	)
	if err != nil {
		t.Logf("authorize unknown user returned expected HTTP/API error: %v", err)
		return
	}
	if len(resp.Details) == 0 {
		t.Fatal("authorize unknown user returned no details and no error")
	}

	detail := resp.Details[0]
	if detail.ResponseCode == 0 && detail.Status == TransactionStatusAuthorized {
		t.Fatalf("expected authorization rejection for unknown user, got approved detail: %+v", detail)
	}
}

func TestIntegrationOneclickStatusUnknownBuyOrder(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	svc := newIntegrationService(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	_, err := svc.Status(ctx, randomBuyOrder("unknown"))
	if err == nil {
		t.Fatal("expected error for unknown buy_order, got nil")
	}
	t.Logf("status for unknown buy_order returned expected error: %v", err)
}

func TestIntegrationOneclickRefundUnknownBuyOrder(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	svc := newIntegrationService(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	_, err := svc.Refund(ctx, randomBuyOrder("unknown"), cfg.ChildCode, randomBuyOrder("child"), cfg.Amount)
	if err == nil {
		t.Fatal("expected refund error for unknown buy_order")
	}
	t.Logf("refund unknown transaction returned expected error: %v", err)
}

func TestIntegrationOneclickCaptureUnknown(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	svc := newIntegrationService(t, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	_, err := svc.Capture(ctx, cfg.ChildCode, randomBuyOrder("capture"), "1213", cfg.Amount)
	if err == nil {
		t.Fatal("expected capture error for unknown transaction")
	}
	t.Logf("capture unknown transaction returned expected error: %v", err)
}

func TestIntegrationOneclickAuthorizeStatusRefundHappyPath(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	if !cfg.RunMutating {
		t.Skip("set TRANSBANK_TEST_RUN_MUTATING=1 to run mutating integration tests")
	}
	if cfg.TbkUser == "" || cfg.Username == "" {
		t.Skip("set TRANSBANK_TEST_TBK_USER and TRANSBANK_TEST_USERNAME to run authorize/status/refund happy path")
	}

	svc := newIntegrationService(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	parentBuyOrder := randomBuyOrder("parent")
	childBuyOrder := randomBuyOrder("child")

	authResp, err := svc.Authorize(ctx,
		cfg.Username,
		cfg.TbkUser,
		parentBuyOrder,
		[]TransactionDetail{
			{
				CommerceCode:       cfg.ChildCode,
				BuyOrder:           childBuyOrder,
				Amount:             cfg.Amount,
				InstallmentsNumber: 0,
			},
		},
	)
	if err != nil {
		t.Fatalf("authorize failed: %v", err)
	}
	if len(authResp.Details) == 0 {
		t.Fatal("authorize returned empty details")
	}

	detail := authResp.Details[0]
	if detail.ResponseCode != 0 || detail.Status != TransactionStatusAuthorized {
		t.Fatalf("authorize detail not approved: status=%s response_code=%d", detail.Status, detail.ResponseCode)
	}

	statusResp, err := svc.Status(ctx, parentBuyOrder)
	if err != nil {
		t.Fatalf("status failed after authorize: %v", err)
	}
	if len(statusResp.Details) == 0 {
		t.Fatal("status returned empty details")
	}

	refundResp, err := svc.Refund(ctx, parentBuyOrder, cfg.ChildCode, childBuyOrder, cfg.Amount)
	if err != nil {
		t.Fatalf("refund failed: %v", err)
	}
	if refundResp.Type != "REVERSED" && refundResp.Type != "NULLIFIED" {
		t.Fatalf("unexpected refund type: %s", refundResp.Type)
	}
}

func TestIntegrationOneclickCaptureWithFixture(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	if !cfg.RunMutating {
		t.Skip("set TRANSBANK_TEST_RUN_MUTATING=1 to run mutating integration tests")
	}
	if cfg.CaptureCode == "" || cfg.CaptureOrder == "" || cfg.CaptureAuth == "" || cfg.CaptureAmount <= 0 {
		t.Skip("set capture fixtures (TRANSBANK_TEST_CAPTURE_*) to run capture integration test")
	}

	svc := newIntegrationService(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	resp, err := svc.Capture(ctx, cfg.CaptureCode, cfg.CaptureOrder, cfg.CaptureAuth, cfg.CaptureAmount)
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if resp.ResponseCode != 0 {
		t.Fatalf("capture rejected: response_code=%d", resp.ResponseCode)
	}
	if resp.CapturedAmount <= 0 {
		t.Fatalf("unexpected captured_amount: %d", resp.CapturedAmount)
	}
}

func loadIntegrationConfig(t *testing.T) integrationConfig {
	t.Helper()
	if os.Getenv("TRANSBANK_RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("integration tests disabled: set TRANSBANK_RUN_INTEGRATION_TESTS=1")
	}

	cfg := integrationConfig{
		BaseURL:      getEnv("TRANSBANK_BASE_URL", defaultIntegrationBaseURL),
		CommerceCode: getEnv("TRANSBANK_COMMERCE_CODE", DefaultIntegrationCommerceCode),
		APISecret:    getEnv("TRANSBANK_API_SECRET", DefaultIntegrationAPISecret),
		ResponseURL:  getEnv("TRANSBANK_RESPONSE_URL", defaultIntegrationResponseURL),
		EmailDomain:  getEnv("TRANSBANK_TEST_EMAIL_DOMAIN", "example.com"),
		ChildCode:    getEnv("TRANSBANK_TEST_CHILD_COMMERCE_CODE", defaultIntegrationChildCode),
		RunMutating:  os.Getenv("TRANSBANK_TEST_RUN_MUTATING") == "1",
		Username:     strings.TrimSpace(os.Getenv("TRANSBANK_TEST_USERNAME")),
		TbkUser:      strings.TrimSpace(os.Getenv("TRANSBANK_TEST_TBK_USER")),
		FinishToken:  strings.TrimSpace(os.Getenv("TRANSBANK_TEST_FINISH_TOKEN")),

		RemoveUsername: strings.TrimSpace(os.Getenv("TRANSBANK_TEST_REMOVE_USERNAME")),
		RemoveTbkUser:  strings.TrimSpace(os.Getenv("TRANSBANK_TEST_REMOVE_TBK_USER")),

		CaptureCode:  strings.TrimSpace(os.Getenv("TRANSBANK_TEST_CAPTURE_COMMERCE_CODE")),
		CaptureOrder: strings.TrimSpace(os.Getenv("TRANSBANK_TEST_CAPTURE_BUY_ORDER")),
		CaptureAuth:  strings.TrimSpace(os.Getenv("TRANSBANK_TEST_CAPTURE_AUTH_CODE")),
	}

	amount, err := strconv.Atoi(getEnv("TRANSBANK_TEST_AMOUNT", "1000"))
	if err != nil || amount <= 0 {
		t.Fatalf("invalid TRANSBANK_TEST_AMOUNT: %v", err)
	}
	cfg.Amount = amount

	captureAmountValue := strings.TrimSpace(os.Getenv("TRANSBANK_TEST_CAPTURE_AMOUNT"))
	if captureAmountValue != "" {
		captureAmount, parseErr := strconv.Atoi(captureAmountValue)
		if parseErr != nil {
			t.Fatalf("invalid TRANSBANK_TEST_CAPTURE_AMOUNT: %v", parseErr)
		}
		cfg.CaptureAmount = captureAmount
	}

	return cfg
}

func newIntegrationService(t *testing.T, cfg integrationConfig) *OneclickService {
	t.Helper()
	svc, err := newOneclickServiceWithBaseURL(
		EnvironmentIntegration,
		cfg.CommerceCode,
		cfg.APISecret,
		cfg.BaseURL,
		nil,
	)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return svc
}

func randomBuyOrder(prefix string) string {
	value := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	if len(value) > maxBuyOrderLength {
		return value[:maxBuyOrderLength]
	}
	return value
}

func randomUsername(prefix string) string {
	value := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	if len(value) > maxUsernameLength {
		return value[:maxUsernameLength]
	}
	return value
}

func randomTbkUser() string {
	value := fmt.Sprintf("it-fake-%d", time.Now().UnixNano())
	if len(value) > maxTbkUserLength {
		return value[:maxTbkUserLength]
	}
	return value
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
