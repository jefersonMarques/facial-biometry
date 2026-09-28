package decoder

import (
	"crypto/sha256"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
	"math/big"
)

var brainpoolP256r1OID = asn1.ObjectIdentifier{1, 3, 36, 3, 3, 2, 8, 1, 1, 7}

type subjectPublicKeyInfo struct {
	Algorithm pkix.AlgorithmIdentifier
	PublicKey asn1.BitString
}

type ecdsaASN1Signature struct {
	R *big.Int
	S *big.Int
}

type primeCurve struct {
	p  *big.Int
	a  *big.Int
	b  *big.Int
	gx *big.Int
	gy *big.Int
	n  *big.Int
}

type affinePoint struct {
	x        *big.Int
	y        *big.Int
	infinity bool
}

func verifyBrainpoolP256r1(der []byte, data []byte, signature []byte) (bool, error) {
	var publicKeyInfo subjectPublicKeyInfo
	if rest, err := asn1.Unmarshal(der, &publicKeyInfo); err != nil || len(rest) != 0 {
		return false, nil
	}

	var curveOID asn1.ObjectIdentifier
	if _, err := asn1.Unmarshal(publicKeyInfo.Algorithm.Parameters.FullBytes, &curveOID); err != nil {
		return false, nil
	}
	if !curveOID.Equal(brainpoolP256r1OID) {
		return false, nil
	}

	publicKeyBytes := publicKeyInfo.PublicKey.Bytes
	if len(publicKeyBytes) != 65 || publicKeyBytes[0] != 0x04 {
		return true, errors.New("chave pública Brainpool P256r1 inválida")
	}

	curve := newBrainpoolP256r1()
	publicKey := affinePoint{
		x: new(big.Int).SetBytes(publicKeyBytes[1:33]),
		y: new(big.Int).SetBytes(publicKeyBytes[33:65]),
	}
	if !curve.isOnCurve(publicKey) {
		return true, errors.New("ponto público Brainpool P256r1 fora da curva")
	}

	var parsedSignature ecdsaASN1Signature
	if rest, err := asn1.Unmarshal(signature, &parsedSignature); err != nil || len(rest) != 0 {
		return true, errors.New("assinatura ECDSA ASN.1 inválida")
	}
	if parsedSignature.R == nil || parsedSignature.S == nil {
		return true, errors.New("assinatura ECDSA sem valores R/S")
	}
	if parsedSignature.R.Sign() <= 0 || parsedSignature.S.Sign() <= 0 || parsedSignature.R.Cmp(curve.n) >= 0 || parsedSignature.S.Cmp(curve.n) >= 0 {
		return true, errors.New("assinatura ECDSA fora do intervalo válido")
	}

	digest := sha256.Sum256(data)
	if !curve.verify(publicKey, digest[:], parsedSignature.R, parsedSignature.S) {
		return true, errors.New("assinatura ECDSA Brainpool P256r1 inválida")
	}

	return true, nil
}

func newBrainpoolP256r1() primeCurve {
	return primeCurve{
		p:  mustBigInt("A9FB57DBA1EEA9BC3E660A909D838D726E3BF623D52620282013481D1F6E5377"),
		a:  mustBigInt("7D5A0975FC2C3057EEF67530417AFFE7FB8055C126DC5C6CE94A4B44F330B5D9"),
		b:  mustBigInt("26DC5C6CE94A4B44F330B5D9BBD77CBF958416295CF7E1CE6BCCDC18FF8C07B6"),
		gx: mustBigInt("8BD2AEB9CB7E57CB2C4B482FFC81B7AFB9DE27E1E3BD23C23A4453BD9ACE3262"),
		gy: mustBigInt("547EF835C3DAC4FD97F8461A14611DC9C27745132DED8E545C1D54C72F046997"),
		n:  mustBigInt("A9FB57DBA1EEA9BC3E660A909D838D718C397AA3B561A6F7901E0E82974856A7"),
	}
}

func mustBigInt(value string) *big.Int {
	result, ok := new(big.Int).SetString(value, 16)
	if !ok {
		panic(fmt.Sprintf("invalid curve parameter: %s", value))
	}
	return result
}

func (curve primeCurve) isOnCurve(point affinePoint) bool {
	if point.infinity || point.x == nil || point.y == nil {
		return false
	}
	if point.x.Sign() < 0 || point.x.Cmp(curve.p) >= 0 || point.y.Sign() < 0 || point.y.Cmp(curve.p) >= 0 {
		return false
	}

	left := new(big.Int).Mul(point.y, point.y)
	left.Mod(left, curve.p)

	right := new(big.Int).Mul(point.x, point.x)
	right.Mul(right, point.x)
	ax := new(big.Int).Mul(curve.a, point.x)
	right.Add(right, ax)
	right.Add(right, curve.b)
	right.Mod(right, curve.p)

	return left.Cmp(right) == 0
}

func (curve primeCurve) verify(publicKey affinePoint, digest []byte, r *big.Int, s *big.Int) bool {
	e := new(big.Int).SetBytes(digest)
	if excess := e.BitLen() - curve.n.BitLen(); excess > 0 {
		e.Rsh(e, uint(excess))
	}

	w := new(big.Int).ModInverse(s, curve.n)
	if w == nil {
		return false
	}

	u1 := new(big.Int).Mul(e, w)
	u1.Mod(u1, curve.n)
	u2 := new(big.Int).Mul(r, w)
	u2.Mod(u2, curve.n)

	generator := affinePoint{x: curve.gx, y: curve.gy}
	point := curve.add(curve.scalarMult(generator, u1), curve.scalarMult(publicKey, u2))
	if point.infinity {
		return false
	}

	x := new(big.Int).Mod(point.x, curve.n)
	return x.Cmp(r) == 0
}

func (curve primeCurve) scalarMult(point affinePoint, scalar *big.Int) affinePoint {
	result := affinePoint{infinity: true}
	for bit := scalar.BitLen() - 1; bit >= 0; bit-- {
		result = curve.double(result)
		if scalar.Bit(bit) == 1 {
			result = curve.add(result, point)
		}
	}
	return result
}

func (curve primeCurve) add(left affinePoint, right affinePoint) affinePoint {
	if left.infinity {
		return clonePoint(right)
	}
	if right.infinity {
		return clonePoint(left)
	}

	if left.x.Cmp(right.x) == 0 {
		ySum := new(big.Int).Add(left.y, right.y)
		ySum.Mod(ySum, curve.p)
		if ySum.Sign() == 0 {
			return affinePoint{infinity: true}
		}
		return curve.double(left)
	}

	numerator := new(big.Int).Sub(right.y, left.y)
	numerator.Mod(numerator, curve.p)
	denominator := new(big.Int).Sub(right.x, left.x)
	denominator.Mod(denominator, curve.p)
	denominatorInverse := new(big.Int).ModInverse(denominator, curve.p)
	if denominatorInverse == nil {
		return affinePoint{infinity: true}
	}

	lambda := new(big.Int).Mul(numerator, denominatorInverse)
	lambda.Mod(lambda, curve.p)

	x := new(big.Int).Mul(lambda, lambda)
	x.Sub(x, left.x)
	x.Sub(x, right.x)
	x.Mod(x, curve.p)

	y := new(big.Int).Sub(left.x, x)
	y.Mul(lambda, y)
	y.Sub(y, left.y)
	y.Mod(y, curve.p)

	return affinePoint{x: x, y: y}
}

func (curve primeCurve) double(point affinePoint) affinePoint {
	if point.infinity || point.y.Sign() == 0 {
		return affinePoint{infinity: true}
	}

	numerator := new(big.Int).Mul(point.x, point.x)
	numerator.Mul(numerator, big.NewInt(3))
	numerator.Add(numerator, curve.a)
	numerator.Mod(numerator, curve.p)

	denominator := new(big.Int).Lsh(new(big.Int).Set(point.y), 1)
	denominator.Mod(denominator, curve.p)
	denominatorInverse := new(big.Int).ModInverse(denominator, curve.p)
	if denominatorInverse == nil {
		return affinePoint{infinity: true}
	}

	lambda := new(big.Int).Mul(numerator, denominatorInverse)
	lambda.Mod(lambda, curve.p)

	x := new(big.Int).Mul(lambda, lambda)
	twoX := new(big.Int).Lsh(new(big.Int).Set(point.x), 1)
	x.Sub(x, twoX)
	x.Mod(x, curve.p)

	y := new(big.Int).Sub(point.x, x)
	y.Mul(lambda, y)
	y.Sub(y, point.y)
	y.Mod(y, curve.p)

	return affinePoint{x: x, y: y}
}

func clonePoint(point affinePoint) affinePoint {
	if point.infinity {
		return affinePoint{infinity: true}
	}
	return affinePoint{x: new(big.Int).Set(point.x), y: new(big.Int).Set(point.y)}
}
