//go:build ignore

package main

import (
	"log"
	"net/http"

	"github.com/benaskins/axon"
	gate "github.com/benaskins/axon-gate"
	"github.com/benaskins/axon-gate/gatetest"
)

func main() {
	store := gatetest.NewMemoryApprovalStore()
	signal := gate.NewSignalClient("http://localhost:8080", "+61400000000")
	authClient := axon.NewAuthClient("http://localhost:9090")

	handler := gate.NewHandler(store, signal, authClient,
		"https://gate.studio.internal",
		"https://auth.studio.internal/login",
	)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/approvals", handler.CreateApproval)
	mux.HandleFunc("GET /api/approvals/{id}", handler.GetApproval)
	mux.HandleFunc("POST /api/notifications", handler.SendNotification)
	mux.HandleFunc("GET /approve/{id}", handler.ShowApprovalPage)
	mux.HandleFunc("POST /approve/{id}", handler.ProcessApproval)

	log.Println("gate listening on :8800")
	log.Fatal(http.ListenAndServe(":8800", mux))
}
