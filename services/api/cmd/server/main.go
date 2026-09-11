package main

import (
	"log"
	"net/http"

	"faceproof/services/api/internal/config"
	"faceproof/services/api/internal/engine"
	"faceproof/services/api/internal/httpapi"
	"faceproof/services/api/internal/security"
	"faceproof/services/api/internal/session"
	templaterepository "faceproof/services/api/internal/template"
)

func main() {
	configuration, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	templates, err := templaterepository.NewRepository(configuration.TemplateDirectory, configuration.TemplateKey)
	if err != nil {
		log.Fatal(err)
	}

	handler := httpapi.NewHandler(
		configuration,
		session.NewStore(),
		security.NewSigner(configuration.SessionSecret),
		engine.NewClient(configuration.EngineURL),
		templates,
	)

	server := &http.Server{
		Addr:              configuration.APIAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5_000_000_000,
		ReadTimeout:       35_000_000_000,
		WriteTimeout:      35_000_000_000,
		IdleTimeout:       60_000_000_000,
	}

	log.Printf("FaceProof API listening on %s", configuration.APIAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
