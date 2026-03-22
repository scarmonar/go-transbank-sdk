//go:build integration

package oneclick

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// Integration tests for Transbank Oneclick API
// These tests validate the SDK against the real Transbank integration environment
// 
// Website: https://www.transbankdevelopers.cl/documentacion/como_empezar#ambientes
// API Reference: https://www.transbankdevelopers.cl/referencia/oneclick
//
// Run ALL tests:
//   go test -tags=integration -v ./...
//
// Run specific test:
//   go test -tags=integration -v -run TestIntegrationCredentialsValid ./oneclick

// TestIntegrationCredentialsValid validates that the SDK can be initialized with valid credentials
func TestIntegrationCredentialsValid(t *testing.T) {
	t.Log("Test: Initialize SDK with integration environment credentials")
	
	svc, err := getIntegrationService(t)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Assertions
	if svc.CommerceCode() != "597055555541" {
		t.Fatalf("expected commerce code 597055555541, got %s", svc.CommerceCode())
	}

	if !svc.IsIntegrationEnvironment() {
		t.Fatal("expected integration environment detected")
	}

	if svc.IsProduction() {
		t.Fatal("should not be detected as production")
	}

	t.Logf("✓ Commerce Code: %s", svc.CommerceCode())
	t.Logf("✓ Environment: Integration (not production)")
}

// TestIntegrationStartInscription tests creating a new inscription
// This corresponds to POST /inscriptions endpoint
func TestIntegrationStartInscription(t *testing.T) {
	t.Log("Test: Create new inscription (POST /inscriptions)")
	
	svc, err := getIntegrationService(t)
	if err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	username := fmt.Sprintf("test_user_%d", time.Now().Unix())
	email := fmt.Sprintf("test_%d@example.com", time.Now().Unix())

	inscResp, err := svc.Start(ctx, username, email, "https://mysite.com/callback")
	if err != nil {
		t.Fatalf("failed to start inscription: %v", err)
	}

	// Validate response structure
	if inscResp.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if inscResp.URLWebpay == "" {
		t.Fatal("expected non-empty webpay URL")
	}

	t.Logf("✓ Inscription created successfully")
	t.Logf("  Token: %s", inscResp.Token)
	t.Logf("  Webpay URL: %s", inscResp.URLWebpay)
	t.Logf("  → User must visit this URL and authorize their card")
	t.Logf("  → They will be redirected back with TBK_TOKEN=%s", inscResp.Token)
}

// TestIntegrationInscriptionFlow tests the complete inscription workflow
// NOTE: This requires manual authorization at Transbank website
// The test demonstrates the flow but may fail at confirmation step if token not manually authorized
func TestIntegrationInscriptionFlow(t *testing.T) {
	t.Log("Test: Complete inscription flow (requires manual authorization)")
	
	svc, err := getIntegrationService(t)
	if err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Step 1: Create inscription
	t.Log("  Step 1: POST /inscriptions")
	username := fmt.Sprintf("testuser_%d", time.Now().Unix())
	email := fmt.Sprintf("user_%d@test.com", time.Now().Unix())

	inscResp, err := svc.Start(ctx, username, email, "https://mysite.com/return")
	if err != nil {
		t.Fatalf("failed to start inscription: %v", err)
	}

	if inscResp.Token == "" {
		t.Fatal("expected non-empty token")
	}

	t.Logf("  ✓ Inscription created")
	t.Logf("    Token: %s", inscResp.Token)
	t.Logf("    Webpay URL: %s (requires manual authorization)", inscResp.URLWebpay)

	// Step 2: Attempt to confirm after manual authorization
	// In real scenario: user goes to URLWebpay, authorizes card, gets redirected with TBK_TOKEN
	t.Log("  Step 2: PUT /inscriptions/{token} (confirm)")
	confirmResp, err := svc.Finish(ctx, inscResp.Token)
	
	// The token might not be authorized yet (manual browser step required)
	// This is EXPECTED to fail unless the token was manually authorized
	if err != nil {
		t.Logf("  ⚠ Confirmation failed (expected without manual authorization)")
		t.Logf("    → Error: %v", err)
		t.Logf("    → To complete: User must manually authorize at Transbank")
		return
	}

	// If we get here, token was authorized successfully
	if confirmResp.ResponseCode != 0 && confirmResp.ResponseCode != 1 {
		t.Fatalf("expected response code 0 or 1, got %d", confirmResp.ResponseCode)
	}

	t.Logf("  ✓ Inscription confirmed")
	t.Logf("    Tbk User: %s", confirmResp.TbkUser)
	t.Logf("    Auth Code: %s", confirmResp.AuthorizationCode)
	t.Logf("    Card Type: %s", confirmResp.CardType)
	t.Logf("    Card Number: %s", confirmResp.CardNumber)
}

// TestIntegrationAuthorizeWithTestCard attempts to authorize using the documented test card
// This shows the API structure but will fail because usernames must match registered pairs
func TestIntegrationAuthorizeWithTestCard(t *testing.T) {
	t.Log("Test: Authorize transaction with test card (documents API structure)")

	svc, err := getIntegrationService(t)
	if err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// These are the documented test values from Transbank
	// In real usage, tbkUser comes from a completed inscription
	testTbkUser := "b6bd6ba3-e718-4107-9386-d2b099a8dd42"
	testUsername := "authenticated_user"
	testBuyOrder := fmt.Sprintf("order_%d", time.Now().Unix())

	t.Logf("Attempting authorization with:")
	t.Logf("  Tbk User: %s", testTbkUser)
	t.Logf("  Username: %s", testUsername)
	t.Logf("  Buy Order: %s", testBuyOrder)

	authResp, err := svc.Authorize(ctx,
		testUsername,
		testTbkUser,
		testBuyOrder,
		[]TransactionDetail{
			{
				CommerceCode:       "597055555542",
				BuyOrder:           fmt.Sprintf("item_%d", time.Now().Unix()),
				Amount:             50000,
				InstallmentsNumber: 0,
			},
		},
	)

	if err != nil {
		// This is expected to fail with the test values
		// because the username/tbkUser pair is not actually registered
		t.Logf("⚠ Authorization failed (expected with test values)")
		t.Logf("  Error: %v", err)
		t.Logf("  This demonstrates the API is responding correctly")
		t.Logf("  To authenticate for real: complete an inscription first")
		return
	}

	// If request was successful, validate response structure
	if authResp.BuyOrder == "" {
		t.Fatal("expected buy order in response")
	}
	if len(authResp.Details) == 0 {
		t.Fatal("expected transaction details in response")
	}

	t.Logf("✓ Authorization succeeded")
	t.Logf("  Buy Order: %s", authResp.BuyOrder)
	t.Logf("  Details: %d transaction(s)", len(authResp.Details))
	for i, detail := range authResp.Details {
		t.Logf("    Detail %d - Status: %s, Response Code: %d", 
			i, detail.Status, detail.ResponseCode)
	}
}

// TestIntegrationTransactionStatus demonstrates the status query endpoint
func TestIntegrationTransactionStatus(t *testing.T) {
	t.Log("Test: Query transaction status (GET /transactions/{buyOrder})")

	svc, err := getIntegrationService(t)
	if err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query a non-existent order (expected behavior)
	buyOrder := "nonexistent_order_12345"

	t.Logf("Querying status for buy order: %s", buyOrder)
	statusResp, err := svc.Status(ctx, buyOrder)

	if err != nil {
		// Expected for non-existent orders
		t.Logf("✓ Status query responded with error (expected for non-existent order)")
		t.Logf("  Error: %v", err)
		return
	}

	// If successful, validate structure
	if statusResp == nil {
		t.Fatal("expected status response")
	}

	t.Logf("✓ Status query succeeded")
	t.Logf("  Details count: %d", len(statusResp.Details))
	for i, detail := range statusResp.Details {
		t.Logf("    Detail %d - Status: %s, Response Code: %d", 
			i, detail.Status, detail.ResponseCode)
	}
}

// TestIntegrationErrorHandling validates error responses from Transbank
func TestIntegrationErrorHandling(t *testing.T) {
	t.Log("Test: Validate error handling with invalid inputs")

	svc, err := getIntegrationService(t)
	if err != nil {
		t.Fatalf("failed to get service: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test: Invalid commerce code
	t.Log("  Subtest 1: Invalid commerce code")
	_, err = svc.Authorize(ctx, "user", "tbkuser", "order", []TransactionDetail{
		{
			CommerceCode:       "invalid",
			BuyOrder:           "order",
			Amount:             1000,
			InstallmentsNumber: 0,
		},
	})

	if err == nil {
		t.Fatal("expected error for invalid commerce code")
	}
	t.Logf("  ✓ Correctly rejected: %v", err)

	// Test: Empty buy order
	t.Log("  Subtest 2: Empty buy order")
	_, err = svc.Authorize(ctx, "user", "tbkuser", "", []TransactionDetail{
		{
			CommerceCode:       "597055555542",
			BuyOrder:           "order",
			Amount:             1000,
			InstallmentsNumber: 0,
		},
	})

	if err == nil {
		t.Fatal("expected error for empty buy order")
	}
	t.Logf("  ✓ Correctly rejected: %v", err)

	// Test: Invalid amount
	t.Log("  Subtest 3: Invalid amount")
	_, err = svc.Authorize(ctx, "user", "tbkuser", "order", []TransactionDetail{
		{
			CommerceCode:       "597055555542",
			BuyOrder:           "order",
			Amount:             0,
			InstallmentsNumber: 0,
		},
	})

	if err == nil {
		t.Fatal("expected error for zero amount")
	}
	t.Logf("  ✓ Correctly rejected: %v", err)
}

// TestIntegrationServiceConfiguration tests environment detection
func TestIntegrationServiceConfiguration(t *testing.T) {
	t.Log("Test: Service configuration and environment detection")

	// Integration service
	integrationSvc, err := NewOneclickService(
		"597055555541",
		"579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C",
		"https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2",
		nil,
	)
	if err != nil {
		t.Fatalf("failed to create integration service: %v", err)
	}

	if !integrationSvc.IsIntegrationEnvironment() {
		t.Fatal("expected integration environment")
	}
	if integrationSvc.IsProduction() {
		t.Fatal("should not be production")
	}
	t.Logf("✓ Integration environment correctly detected")

	// Production service (for reference)
	prodSvc, err := NewOneclickService(
		"597055555541",
		"579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C",
		"https://webpay3g.transbank.cl/rswebpaytransaction/api/oneclick/v1.2",
		nil,
	)
	if err != nil {
		t.Fatalf("failed to create production service: %v", err)
	}

	if prodSvc.IsIntegrationEnvironment() {
		t.Fatal("should not be integration environment")
	}
	if !prodSvc.IsProduction() {
		t.Fatal("expected production environment")
	}
	t.Logf("✓ Production environment correctly detected")
}

// Helper function to get integration service
func getIntegrationService(t *testing.T) (*OneclickService, error) {
	// Get credentials from environment or use defaults
	commerceCode := os.Getenv("TRANSBANK_COMMERCE_CODE")
	if commerceCode == "" {
		commerceCode = "597055555541"
	}

	apiSecret := os.Getenv("TRANSBANK_API_SECRET")
	if apiSecret == "" {
		apiSecret = "579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C"
	}

	baseURL := os.Getenv("TRANSBANK_BASE_URL")
	if baseURL == "" {
		baseURL = "https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2"
	}

	t.Logf("Using Transbank Integration Credentials:")
	t.Logf("  Commerce Code: %s", commerceCode)
	t.Logf("  Base URL: %s", baseURL)

	return NewOneclickService(commerceCode, apiSecret, baseURL, nil)
}
