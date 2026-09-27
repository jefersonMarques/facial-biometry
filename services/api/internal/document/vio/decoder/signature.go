package decoder

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
)

type CryptoVerifier struct{}

func NewCryptoVerifier() CryptoVerifier {
	return CryptoVerifier{}
}

func (CryptoVerifier) Verify(data []byte, signature []byte, certificate Certificate) (string, error) {
	der, err := base64.StdEncoding.DecodeString(certificate.PublicKey)
	if err != nil {
		return "", fmt.Errorf("chave pública Base64 inválida: %w", err)
	}

	publicKey, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		handled, brainpoolErr := verifyBrainpoolP256r1(der, data, signature)
		if handled {
			if brainpoolErr != nil {
				return "SHA256withECDSA/brainpoolP256r1", brainpoolErr
			}
			return "SHA256withECDSA/brainpoolP256r1", nil
		}
		return "", fmt.Errorf("chave pública X.509 inválida: %w", err)
	}

	digest := sha256.Sum256(data)

	switch key := publicKey.(type) {
	case *rsa.PublicKey:
		if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
			return "SHA256withRSA", err
		}
		return "SHA256withRSA", nil
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(key, digest[:], signature) {
			return "SHA256withECDSA", errors.New("assinatura ECDSA inválida")
		}
		return "SHA256withECDSA", nil
	default:
		return "", fmt.Errorf("tipo de chave pública não suportado: %T", publicKey)
	}
}
