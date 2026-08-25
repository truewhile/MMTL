package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ShukeBta/MMTL/internal/config"
)

func makeTestPairPEM(t *testing.T) (certPEM, keyPEM string) {
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

func newTestServerManager(t *testing.T) *serverManager {
	t.Helper()
	cfg := &config.Config{}
	cfg.App.Port = 18081
	return newServerManager(cfg, zap.NewNop(), http.NewServeMux())
}

func TestDesiredPairModes(t *testing.T) {
	m := newTestServerManager(t)

	if p, err := m.desiredPair(); err != nil || p != nil {
		t.Fatalf("disabled should be nil pair, got p=%v err=%v", p, err)
	}

	certPEM, keyPEM := makeTestPairPEM(t)
	m.cfg.App.HTTPSEnabled = true
	m.cfg.App.SSLCert, m.cfg.App.SSLKey = certPEM, keyPEM
	p, err := m.desiredPair()
	if err != nil || p == nil || p.version == "" {
		t.Fatalf("content pair failed: p=%v err=%v", p, err)
	}

	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, []byte(certPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte(keyPEM), 0o600); err != nil {
		t.Fatal(err)
	}
	m.cfg.App.SSLCert, m.cfg.App.SSLKey = "", ""
	m.cfg.App.SSLCertPath, m.cfg.App.SSLKeyPath = certPath, keyPath
	p2, err := m.desiredPair()
	if err != nil || p2 == nil {
		t.Fatalf("path pair failed: %v", err)
	}

	m.cfg.App.SSLKeyPath = filepath.Join(dir, "nope.pem")
	if _, err := m.desiredPair(); err == nil {
		t.Fatal("expected error when key file missing")
	}
	m.cfg.App.SSLKeyPath = keyPath

	// 替换文件（换一套新的有效证书）后版本号应变化，触发热更新。
	newCert, newKey := makeTestPairPEM(t)
	if err := os.WriteFile(certPath, []byte(newCert), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte(newKey), 0o600); err != nil {
		t.Fatal(err)
	}
	p3, err := m.desiredPair()
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if p3.version == p2.version {
		t.Fatal("version should change after files replaced")
	}
}
