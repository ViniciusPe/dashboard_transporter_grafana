package main

import (
	"log"
	nethttp "net/http"
	"os"

	apphttp "dashboard-transporter/internal/http"
	"dashboard-transporter/internal/config"
)

func main() {
	cfg := config.Load()

	router := apphttp.NewRouter(cfg)

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	log.Printf("Dashboard Transporter Backend listening on %s", addr)

	// ✅ O router já tem CORS via r.Use(...) dentro do NewRouter
	if err := nethttp.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
