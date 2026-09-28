package main

import (
	"log"
	"net/http"
	"time"

	"faceproof/services/api/internal/config"
	"faceproof/services/api/internal/document/cnh"
	viodata "faceproof/services/api/internal/document/vio/data"
	"faceproof/services/api/internal/document/vio/decoder"
	"faceproof/services/api/internal/document/vio/imagepreview"
	"faceproof/services/api/internal/engine"
	"faceproof/services/api/internal/httpapi"
	"faceproof/services/api/internal/identity"
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
	identityChecks, err := identity.NewRepository(configuration.IdentityDirectory, configuration.IdentityStoreKey)
	if err != nil {
		log.Fatal(err)
	}

	vioRepository := viodata.NewRepository()
	vioService := decoder.NewService(vioRepository, decoder.NewCryptoVerifier())
	previewService := imagepreview.NewService(imagepreview.NewBPGDecoder(configuration.BPGDecoderPath))
	if !previewService.Available() {
		log.Printf("identity CNH photo conversion unavailable until bpgdec is installed/configured")
	}

	engineClient := engine.NewClient(configuration.EngineURL)
	handler := httpapi.NewHandler(
		configuration,
		session.NewStore(),
		security.NewSigner(configuration.SessionSecret),
		engineClient,
		templates,
		identityChecks,
		cnh.NewService(vioService, previewService),
	)

	server := &http.Server{
		Addr:              configuration.APIAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       35 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("FaceProof API listening on %s", configuration.APIAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
