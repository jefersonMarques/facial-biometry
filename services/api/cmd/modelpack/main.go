package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"faceproof/services/api/internal/modelpack"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "keygen":
		err = runKeygen(os.Args[2:])
	case "create":
		err = runCreate(os.Args[2:])
	case "verify":
		err = runVerify(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "modelpack:", err)
		os.Exit(1)
	}
}

func runKeygen(args []string) error {
	set := flag.NewFlagSet("keygen", flag.ContinueOnError)
	privatePath := set.String("private", "", "private key output path")
	publicPath := set.String("public", "", "public key output path")
	if err := set.Parse(args); err != nil {
		return err
	}
	return modelpack.GenerateKeyPair(*privatePath, *publicPath)
}

func runCreate(args []string) error {
	set := flag.NewFlagSet("create", flag.ContinueOnError)
	output := set.String("output", "", "model pack output path")
	privateKey := set.String("private-key", "", "Ed25519 private key path")
	packID := set.String("pack-id", "faceproof-standard", "model pack id")
	packVersion := set.String("pack-version", "0.1.0", "model pack version")
	secureCoreMinVersion := set.String("secure-core-min-version", "0.2.0", "minimum Secure Core version")
	yunet := set.String("yunet", "", "YuNet ONNX path")
	sface := set.String("sface", "", "SFace ONNX path")
	minifasnet := set.String("minifasnet", "", "MiniFASNet ONNX path")
	if err := set.Parse(args); err != nil {
		return err
	}

	manifest, err := modelpack.Create(modelpack.CreateOptions{
		OutputPath:           *output,
		PrivateKeyPath:       *privateKey,
		PackID:               *packID,
		PackVersion:          *packVersion,
		SecureCoreMinVersion: *secureCoreMinVersion,
		Models: []modelpack.SourceModel{
			{
				Role:    modelpack.RoleYUNet,
				Name:    "YuNet",
				Version: "2023mar",
				Path:    *yunet,
			},
			{
				Role:    modelpack.RoleSFace,
				Name:    "SFace",
				Version: "2021dec",
				Path:    *sface,
			},
			{
				Role:    modelpack.RoleMiniFASNet,
				Name:    "MiniFASNetV2",
				Version: "V2",
				Path:    *minifasnet,
			},
		},
	})
	if err != nil {
		return err
	}

	encoded, _ := json.MarshalIndent(manifest, "", "  ")
	fmt.Println(string(encoded))
	return nil
}

func runVerify(args []string) error {
	set := flag.NewFlagSet("verify", flag.ContinueOnError)
	packPath := set.String("pack", "", "model pack path")
	publicKeyPath := set.String("public-key", "", "Ed25519 public key path")
	cacheDir := set.String("cache-dir", "", "verified extraction directory")
	if err := set.Parse(args); err != nil {
		return err
	}
	publicKeyBytes, err := os.ReadFile(*publicKeyPath)
	if err != nil {
		return fmt.Errorf("read public key: %w", err)
	}

	result, err := modelpack.VerifyAndExtract(
		*packPath,
		strings.TrimSpace(string(publicKeyBytes)),
		*cacheDir,
	)
	if err != nil {
		return err
	}

	fmt.Printf("[ok] pack: %s %s\n", result.Manifest.PackID, result.Manifest.PackVersion)
	fmt.Printf("[ok] manifest SHA-256: %s\n", result.ManifestSHA256)
	fmt.Printf("[ok] cache: %s\n", filepath.Clean(result.ExtractedRoot))
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  modelpack keygen --private <path> --public <path>")
	fmt.Fprintln(os.Stderr, "  modelpack create --output <file.fpmp> --private-key <path> --yunet <onnx> --sface <onnx> --minifasnet <onnx>")
	fmt.Fprintln(os.Stderr, "  modelpack verify --pack <file.fpmp> --public-key <path> --cache-dir <dir>")
}
