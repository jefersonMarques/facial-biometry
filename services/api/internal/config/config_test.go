package config

import "testing"

func TestProductionConfigurationRequiresLoopbackAPI(t *testing.T) {
	t.Parallel()

	configuration := validProductionConfiguration()
	configuration.APIAddress = ":8180"

	if err := validateProductionConfiguration(configuration); err == nil {
		t.Fatal("production must reject wildcard API bind")
	}
}

func TestProductionConfigurationAcceptsLoopbackAPI(t *testing.T) {
	t.Parallel()

	configuration := validProductionConfiguration()
	if err := validateProductionConfiguration(configuration); err != nil {
		t.Fatalf("valid production configuration rejected: %v", err)
	}
}

func TestProductionConfigurationRequiresHTTPSOrigin(t *testing.T) {
	t.Parallel()

	configuration := validProductionConfiguration()
	configuration.AllowedOrigin = "http://faceproof.example.com"

	if err := validateProductionConfiguration(configuration); err == nil {
		t.Fatal("production must reject non-HTTPS allowed origin")
	}
}

func TestProductionConfigurationRequiresHTTPSVerificationURL(t *testing.T) {
	t.Parallel()

	configuration := validProductionConfiguration()
	configuration.IdentityVerifyURL = "http://faceproof.example.com/verify.html"

	if err := validateProductionConfiguration(configuration); err == nil {
		t.Fatal("production must reject non-HTTPS verification URL")
	}
}

func TestProductionConfigurationRequiresNativeCoreAndTenantRegistry(t *testing.T) {
	t.Parallel()

	configuration := validProductionConfiguration()
	configuration.SecureCoreLibrary = ""
	if err := validateProductionConfiguration(configuration); err == nil {
		t.Fatal("production must require Secure Core")
	}

	configuration = validProductionConfiguration()
	configuration.TenantRegistryPath = ""
	if err := validateProductionConfiguration(configuration); err == nil {
		t.Fatal("production must require tenant registry")
	}
}

func TestDevelopmentConfigurationKeepsCurrentFlexibility(t *testing.T) {
	t.Parallel()

	configuration := Config{
		Environment:       "development",
		APIAddress:        ":8180",
		AllowedOrigin:     "http://localhost:5173",
		IdentityVerifyURL: "http://localhost:5173/verify.html",
	}
	if err := validateProductionConfiguration(configuration); err != nil {
		t.Fatalf("development configuration unexpectedly rejected: %v", err)
	}
}

func TestLoopbackListenAddress(t *testing.T) {
	t.Parallel()

	for _, address := range []string{
		"127.0.0.1:8180",
		"[::1]:8180",
		"localhost:8180",
	} {
		if !isLoopbackListenAddress(address) {
			t.Fatalf("expected loopback address: %s", address)
		}
	}

	for _, address := range []string{
		":8180",
		"0.0.0.0:8180",
		"[::]:8180",
		"192.168.1.5:8180",
		"8180",
	} {
		if isLoopbackListenAddress(address) {
			t.Fatalf("expected non-loopback address: %s", address)
		}
	}
}

func validProductionConfiguration() Config {
	return Config{
		Environment:        "production",
		APIAddress:         "127.0.0.1:8180",
		AllowedOrigin:      "https://faceproof.example.com",
		IdentityVerifyURL:  "https://faceproof.example.com/verify.html",
		SecureCoreLibrary:  "/opt/faceproof/lib/libfaceproof_core.so",
		ModelPackPath:      "/opt/faceproof/models/faceproof-standard.fpmp",
		ModelPackPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		TenantRegistryPath: "/etc/faceproof/tenants.json",
	}
}
