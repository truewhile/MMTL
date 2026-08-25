package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeTestKeyPair(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	certPEM = strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
	keyPEM = strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})))
	return certPEM, keyPEM
}

func TestResolveSSLMaterial(t *testing.T) {
	dir := t.TempDir()
	certPEM, _ := makeTestKeyPair(t)
	path := filepath.Join(dir, "cert.pem")
	if err := os.WriteFile(path, []byte(certPEM), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		content string
		path   string
		want   string
		err    bool
	}{
		{name: "content only", content: certPEM, want: certPEM},
		{name: "path only", path: path, want: certPEM},
		{name: "path wins over content", content: "bogus", path: path, want: certPEM},
		{name: "both empty", err: true},
		{name: "missing file", path: filepath.Join(dir, "missing.pem"), err: true},
		{name: "path is dir", path: dir, err: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveSSLMaterial(tc.content, tc.path, "证书")
			if tc.err {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestResolveSSLKeyPair(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := makeTestKeyPair(t)
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, []byte(certPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte(keyPEM), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveSSLKeyPair(certPEM, "", keyPEM, ""); err != nil {
		t.Fatalf("content pair: %v", err)
	}
	if _, err := ResolveSSLKeyPair("", certPath, "", keyPath); err != nil {
		t.Fatalf("path pair: %v", err)
	}
	if _, err := ResolveSSLKeyPair(certPEM, "", "", keyPath); err != nil {
		t.Fatalf("mixed pair: %v", err)
	}

	otherCert, _ := makeTestKeyPair(t)
	if _, err := ResolveSSLKeyPair(otherCert, "", keyPEM, ""); err == nil {
		t.Fatal("expected mismatch error")
	}

	if got, ok := PathFingerprint(certPath); !ok || got == "" {
		t.Fatalf("PathFingerprint failed: got=%q ok=%v", got, ok)
	}
	if _, ok := PathFingerprint(filepath.Join(dir, "missing.pem")); ok {
		t.Fatal("PathFingerprint should report missing file")
	}
	if _, ok := PathFingerprint("   "); ok {
		t.Fatal("empty PathFingerprint should not be ok")
	}
}
