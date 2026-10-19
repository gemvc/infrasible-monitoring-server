package registrar

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"strings"
)

// ClientIdentity is the TLS leaf identity used for ACL lookup.
type ClientIdentity struct {
	FingerprintSHA256 string // lowercase hex, no colons
	Subject           string // logs only
}

// FingerprintSHA256DER returns lowercase hex SHA-256 of certificate DER bytes.
func FingerprintSHA256DER(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// FingerprintFromCert returns a ClientIdentity from a leaf certificate.
func FingerprintFromCert(cert *x509.Certificate) (ClientIdentity, error) {
	if cert == nil {
		return ClientIdentity{}, fmt.Errorf("nil certificate")
	}
	return ClientIdentity{
		FingerprintSHA256: FingerprintSHA256DER(cert.Raw),
		Subject:           cert.Subject.String(),
	}, nil
}

// NormalizeFingerprint lowercases and strips colons/spaces for ACL compare.
func NormalizeFingerprint(fp string) string {
	fp = strings.TrimSpace(strings.ToLower(fp))
	fp = strings.ReplaceAll(fp, ":", "")
	fp = strings.ReplaceAll(fp, " ", "")
	return fp
}
