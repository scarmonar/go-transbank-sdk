package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/scarmonar/go-transbank-sdk/oneclick"
)

// Example: simple Oneclick Mall inscription flow.
// This shows how to:
// 1. Create an inscription
// 2. Redirect user to Transbank
// 3. Confirm the token after redirect
func main() {
	// Initialize service with integration environment
	svc, err := oneclick.NewOneclickService(
		"597055555541",
		"579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C",
		"https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2",
		nil, // Use default HTTP client
	)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("=== Step 1: Create Inscription ===")
	inscResp, err := svc.Start(
		ctx,
		"usuario_123",
		"user@example.com",
		"https://ciertio.app/api/v1/private/billing/confirm-inscription",
	)
	if err != nil {
		log.Fatalf("Failed to create inscription: %v", err)
	}

	fmt.Printf("Token: %s\n", inscResp.Token)
	fmt.Printf("Redirect URL: %s\n", inscResp.URLWebpay)
	fmt.Println("\n→ User should be redirected to the above URL")
	fmt.Println("→ After user authorizes card, they are redirected back with TBK_TOKEN parameter")

	// Simulate user returning from Transbank with the token
	// In a real app, this would come from the redirect URL parameter
	token := inscResp.Token

	fmt.Println("\n=== Step 2: Confirm Inscription ===")
	confirmResp, err := svc.Finish(ctx, token)
	if err != nil {
		log.Fatalf("Failed to confirm inscription: %v", err)
	}

	fmt.Printf("Response Code: %d\n", confirmResp.ResponseCode)
	fmt.Printf("Tbk User: %s\n", confirmResp.TbkUser)
	fmt.Printf("Authorization Code: %s\n", confirmResp.AuthorizationCode)
	fmt.Printf("Card Type: %s\n", confirmResp.CardType)
	fmt.Printf("Card Number: %s\n", confirmResp.CardNumber)
	fmt.Println("\n✓ Card is now registered and ready for charges")
	fmt.Printf("✓ Test charge of $50 CLP was automatically reversed by Transbank")

	// Store confirmResp.TbkUser in your database for future charges
	fmt.Printf("\nStore this for future charges: %s\n", confirmResp.TbkUser)
}
