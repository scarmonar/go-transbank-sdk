package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/scarmonar/go-transbank-sdk/oneclick"
)

// Example: Simple charge flow
// This shows how to:
// 1. Authorize a transaction on a registered card
// 2. Check transaction status
// 3. Reverse/refund a transaction
func main() {
	// Initialize service
	svc, err := oneclick.NewOneclickService(
		"597055555541",
		"579B532A7440BB0C9079DED94D31EA1615BACEB56610332264630D42D0A36B1C",
		"https://webpay3gint.transbank.cl/rswebpaytransaction/api/oneclick/v1.2",
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()






























































}	fmt.Println("\n✓ Transaction reversed successfully")	fmt.Printf("Balance: %d CLP\n", reverseResp.Balance)	fmt.Printf("Nullified Amount: %d CLP\n", reverseResp.NullifiedAmount)	fmt.Printf("Authorization Code: %s\n", reverseResp.AuthorizationCode)	fmt.Printf("Type: %s\n", reverseResp.Type)	}		log.Fatalf("Failed to reverse transaction: %v", err)	if err != nil {	)		50000,                // Amount to refund		"invoice_001",        // Detail buy order		"597055555542",       // Commerce code		"order_592301",       // Buy order		ctx,	reverseResp, err := svc.ReverseTransaction(	fmt.Println("\n=== Reverse Transaction ===")	fmt.Printf("Status: %s\n", statusResp.Details[0].Status)	}		log.Fatalf("Failed to get transaction status: %v", err)	if err != nil {	statusResp, err := svc.GetTransactionStatus(ctx, "order_592301")	fmt.Println("\n=== Get Transaction Status ===")	}		fmt.Printf("  Amount: %d CLP\n", detail.Amount)		fmt.Printf("  Response Code: %d\n", detail.ResponseCode)		fmt.Printf("  Authorization Code: %s\n", detail.AuthorizationCode)		fmt.Printf("  Status: %s\n", detail.Status)		fmt.Printf("\nDetail %d:\n", i)	for i, detail := range authResp.Details {	fmt.Printf("Transaction Date: %v\n", authResp.TransactionDate)	fmt.Printf("Card Number: %s\n", authResp.CardDetail.CardNumber)	fmt.Printf("Buy Order: %s\n", authResp.BuyOrder)	}		log.Fatalf("Failed to authorize transaction: %v", err)	if err != nil {	)		},			},				InstallmentsNumber: 1,				Amount:             50000, // $50k CLP				BuyOrder:           "invoice_001",				CommerceCode:       "597055555542",			{		[]oneclick.TransactionDetail{		"order_592301", // Unique order ID		tbkUser,		username,		ctx,	authResp, err := svc.AuthorizeTransaction(	fmt.Println("=== Authorize a Transaction ===")	username := "usuario_123"	tbkUser := "b6bd6ba3-e718-4107-9386-d2b099a8dd42"	// These values come from the inscription confirmation (previous example)