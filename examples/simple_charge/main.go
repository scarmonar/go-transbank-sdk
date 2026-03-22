package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/scarmonar/go-transbank-sdk/oneclick"
)

// Example: simple Oneclick Mall charge flow.
func main() {
	svc, err := oneclick.NewOneclickService()
	if err != nil {
		log.Fatalf("failed to create service: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	username := "usuario_123"
	tbkUser := "b6bd6ba3-e718-4107-9386-d2b099a8dd42"
	parentBuyOrder := "order_592301"

	fmt.Println("=== Authorize Transaction ===")
	authResp, err := svc.Authorize(ctx,
		username,
		tbkUser,
		parentBuyOrder,
		[]oneclick.TransactionDetail{
			{
				CommerceCode:       "597055555542",
				BuyOrder:           "invoice_001",
				Amount:             50000,
				InstallmentsNumber: 0,
			},
		},
	)
	if err != nil {
		log.Fatalf("failed to authorize transaction: %v", err)
	}

	fmt.Printf("Buy Order: %s\n", authResp.BuyOrder)
	fmt.Printf("Card Number: %s\n", authResp.CardDetail.CardNumber)
	for i, detail := range authResp.Details {
		fmt.Printf("Detail %d Status: %s ResponseCode: %d\n", i, detail.Status, detail.ResponseCode)
	}

	fmt.Println("\n=== Transaction Status ===")
	statusResp, err := svc.Status(ctx, parentBuyOrder)
	if err != nil {
		log.Fatalf("failed to get transaction status: %v", err)
	}
	if len(statusResp.Details) > 0 {
		fmt.Printf("Current status: %s\n", statusResp.Details[0].Status)
	}

	fmt.Println("\n=== Refund Transaction ===")
	refundResp, err := svc.Refund(
		ctx,
		parentBuyOrder,
		"597055555542",
		"invoice_001",
		50000,
	)
	if err != nil {
		log.Fatalf("failed to refund transaction: %v", err)
	}

	fmt.Printf("Refund type: %s\n", refundResp.Type)
	fmt.Printf("Refund response code: %d\n", refundResp.ResponseCode)
}
