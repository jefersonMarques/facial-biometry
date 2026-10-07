package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"faceproof/services/api/internal/analytics"
	"faceproof/services/api/internal/config"
	"faceproof/services/api/internal/document/cnh"
	"faceproof/services/api/internal/document/pdfanalysis"
	viodata "faceproof/services/api/internal/document/vio/data"
	"faceproof/services/api/internal/document/vio/decoder"
	"faceproof/services/api/internal/document/vio/imagepreview"
	"faceproof/services/api/internal/engine"
	"faceproof/services/api/internal/httpapi"
	"faceproof/services/api/internal/identity"
	"faceproof/services/api/internal/modelpack"
	"faceproof/services/api/internal/security"
	"faceproof/services/api/internal/session"
	templaterepository "faceproof/services/api/internal/template"
	"faceproof/services/api/internal/tenant"
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

	var analyticsRepository *analytics.Repository
	if configuration.AnalyticsDatabaseURL != "" {
		analyticsContext, cancelAnalytics := context.WithTimeout(context.Background(), 8*time.Second)
		analyticsRepository, err = analytics.Open(
			analyticsContext,
			configuration.AnalyticsDatabaseURL,
			configuration.AnalyticsSubjectKey,
		)
		cancelAnalytics()
		if err != nil {
			log.Printf("FaceProof analytics unavailable; continuing without analytics: %v", err)
			analyticsRepository = nil
		} else {
			defer analyticsRepository.Close()
			log.Printf("FaceProof analytics enabled")
		}
	}

	prependToolDirectories(configuration.PDFSigPath, configuration.PDFInfoPath, configuration.BPGDecoderPath)
	capabilities := pdfanalysis.DetectCapabilities()
	var missingRequired []string
	if !capabilities.PDFRender {
		missingRequired = append(missingRequired, "pdftoppm")
	}
	if !capabilities.PDFInfo {
		missingRequired = append(missingRequired, "pdfinfo")
	}
	if len(missingRequired) > 0 {
		log.Printf("CNH identity validation unavailable: missing %s", strings.Join(missingRequired, ", "))
	}
	if !capabilities.PDFSig {
		log.Printf("CNH identity note: pdfsig unavailable; native Go PKCS#7/CMS verification will be used")
	}
	if !capabilities.PDFImages {
		log.Printf("CNH identity note: pdfimages unavailable; QR/photo extraction will use rendered-page fallback")
	}

	vioRepository := viodata.NewRepository()
	vioService := decoder.NewService(vioRepository, decoder.NewCryptoVerifier())
	previewService := imagepreview.NewService(imagepreview.NewBPGDecoder(configuration.BPGDecoderPath))
	if !previewService.Available() {
		log.Printf("CNH identity note: bpgdec unavailable; signed-PDF portrait will be preferred")
	}

	engineClient := engine.NewClient(configuration.EngineURL)
	if configuration.SecureCoreLibrary != "" {
		if configuration.ModelPackPath == "" || configuration.ModelPackPublicKey == "" {
			log.Fatal("FaceProof native runtime requires FACEPROOF_MODEL_PACK and FACEPROOF_MODEL_PACK_PUBLIC_KEY")
		}

		verifiedPack, packErr := modelpack.VerifyAndExtract(
			configuration.ModelPackPath,
			configuration.ModelPackPublicKey,
			configuration.ModelPackCacheDirectory,
		)
		if packErr != nil {
			log.Fatalf("FaceProof Model Pack verification failed: %v", packErr)
		}
		if packErr := modelpack.ValidateSecureCoreCompatibility(
			verifiedPack.Manifest,
			engine.CurrentSecureCoreVersion,
		); packErr != nil {
			log.Fatalf("FaceProof Model Pack is incompatible: %v", packErr)
		}

		if err := engineClient.EnableNativeIdentity(engine.NativeConfig{
			LibraryPath:    configuration.SecureCoreLibrary,
			YUNetModelPath: verifiedPack.YUNetPath,
			SFaceModelPath: verifiedPack.SFacePath,
			MiniFASNetPath: verifiedPack.MiniFASNetPath,
		}); err != nil {
			log.Fatalf("FaceProof Secure Core could not start: %v", err)
		}
		defer engineClient.Close()
		log.Printf(
			"FaceProof Model Pack verified: %s %s manifest=%s",
			verifiedPack.Manifest.PackID,
			verifiedPack.Manifest.PackVersion,
			verifiedPack.ManifestSHA256,
		)
		log.Printf("FaceProof Secure Core C++ authority enabled")
	} else {
		log.Printf("FaceProof identity engine: Python authority (development fallback)")
	}

	handler := httpapi.NewHandler(
		configuration,
		session.NewStore(),
		security.NewSigner(configuration.SessionSecret),
		engineClient,
		templates,
		identityChecks,
		cnh.NewService(vioService, previewService),
		analyticsRepository,
	)

	if configuration.TenantRegistryPath != "" {
		tenantRegistry, tenantErr := tenant.Load(configuration.TenantRegistryPath)
		if tenantErr != nil {
			log.Fatalf("FaceProof tenant registry could not load: %v", tenantErr)
		}
		handler.SetTenantRegistry(tenantRegistry)
		log.Printf(
			"FaceProof tenant registry enabled: %d tenant(s)",
			tenantRegistry.Count(),
		)
	} else if len(configuration.IdentityIssuerKey) > 0 {
		log.Printf("FaceProof tenant registry: legacy default tenant fallback enabled")
	} else {
		log.Printf("FaceProof tenant registry: no issuer tenant configured")
	}

	server := &http.Server{
		Addr:              configuration.APIAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       105 * time.Second,
		WriteTimeout:      105 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("FaceProof API listening on %s", configuration.APIAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}


func prependToolDirectories(paths ...string) {
	currentPath := os.Getenv("PATH")
	seen := map[string]bool{}
	var directories []string

	for _, configured := range paths {
		configured = strings.TrimSpace(configured)
		if configured == "" {
			continue
		}
		directory := configured
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			directory = filepath.Dir(configured)
		}
		absolute, err := filepath.Abs(directory)
		if err != nil || seen[absolute] {
			continue
		}
		seen[absolute] = true
		directories = append(directories, absolute)
	}
	if len(directories) == 0 {
		return
	}
	os.Setenv("PATH", strings.Join(directories, string(os.PathListSeparator))+string(os.PathListSeparator)+currentPath)
}
