package service

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveSSLMaterial 解析一份 SSL 材料（证书或私钥）的 PEM 内容：
// 优先读取 path 指向的文件，其次使用内容；两者都为空时返回错误。
// what 用于错误提示（"证书" / "私钥"）。
func ResolveSSLMaterial(content, path, what string) (string, error) {
	p := strings.TrimSpace(path)
	if p != "" {
		if info, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("SSL %s文件不可访问：%s（%v）", what, p, err)
		} else if info.IsDir() {
			return "", fmt.Errorf("SSL %s路径指向的是目录，请填写文件路径：%s", what, p)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("读取 SSL %s文件失败：%s（%v）", what, p, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	c := strings.TrimSpace(content)
	if c == "" {
		return "", fmt.Errorf("SSL %s未配置：请填写内容或文件路径", what)
	}
	return c, nil
}

// ResolveSSLKeyPair 解析证书与私钥（各自支持 内容或路径），校验格式与匹配后
// 返回可用的 tls.Certificate。任何一步失败都会给出明确错误。
func ResolveSSLKeyPair(certContent, certPath, keyContent, keyPath string) (*tls.Certificate, error) {
	certPEM, err := ResolveSSLMaterial(certContent, certPath, "证书")
	if err != nil {
		return nil, err
	}
	keyPEM, err := ResolveSSLMaterial(keyContent, keyPath, "私钥")
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("SSL 证书/私钥无效：%v", err)
	}
	if !sslKeyMatchesCert(cert) {
		return nil, errors.New("SSL 证书与私钥不匹配")
	}
	return &cert, nil
}

// PathFingerprint 返回文件路径的内容指纹（路径 + 大小 + 修改时间），用于检测
// 文件是否被替换过；文件不存在时返回 ("", false)。
func PathFingerprint(p string) (string, bool) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", false
	}
	info, err := os.Stat(p)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("path:%s|size:%d|mtime:%d", filepath.Clean(p), info.Size(), info.ModTime().UnixNano()), true
}

// ValidateSSLCert 校验 s 是一个可解析的 PEM 编码 X.509 证书。
func ValidateSSLCert(s string) error {
	block, _ := pem.Decode([]byte(strings.TrimSpace(s)))
	if block == nil {
		return errors.New("SSL 证书格式无效：未找到 PEM 数据")
	}
	if block.Type != "CERTIFICATE" {
		return fmt.Errorf("SSL 证书格式无效：期望 CERTIFICATE，实际为 %s", block.Type)
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return fmt.Errorf("SSL 证书解析失败：%v", err)
	}
	return nil
}

// ValidateSSLKey 校验 s 是一个可解析的 PEM 编码私钥。
func ValidateSSLKey(s string) error {
	block, _ := pem.Decode([]byte(strings.TrimSpace(s)))
	if block == nil {
		return errors.New("SSL 私钥格式无效：未找到 PEM 数据")
	}
	if _, err := parsePrivateKeyBlock(block); err != nil {
		return fmt.Errorf("SSL 私钥解析失败：%v", err)
	}
	return nil
}

// ValidateSSLKeyPair 校验证书与私钥都存在、可解析且相互匹配。
func ValidateSSLKeyPair(certPEM, keyPEM string) error {
	cert, err := tls.X509KeyPair([]byte(strings.TrimSpace(certPEM)), []byte(strings.TrimSpace(keyPEM)))
	if err != nil {
		return fmt.Errorf("SSL 证书/私钥无效：%v", err)
	}
	if !sslKeyMatchesCert(cert) {
		return errors.New("SSL 证书与私钥不匹配")
	}
	return nil
}

// sslKeyMatchesCert 通过公钥是否一致来判断私钥确实对应证书。
func sslKeyMatchesCert(cert tls.Certificate) bool {
	if len(cert.Certificate) == 0 || cert.PrivateKey == nil {
		return false
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return false
	}
	privPub := publicKeyOf(cert.PrivateKey)
	if privPub == nil {
		return false
	}
	eq, ok := leaf.PublicKey.(interface {
		Equal(x crypto.PublicKey) bool
	})
	return ok && eq.Equal(privPub)
}

// publicKeyOf 从各类私钥中提取对应的公钥。
func publicKeyOf(priv crypto.PrivateKey) crypto.PublicKey {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	case ed25519.PrivateKey:
		return k.Public()
	}
	return nil
}

// parsePrivateKeyBlock 支持 PKCS#8 / PKCS#1 RSA / EC 三种常见私钥格式。
func parsePrivateKeyBlock(block *pem.Block) (crypto.PrivateKey, error) {
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("无法解析私钥（支持 PKCS#8 / PKCS#1 RSA / EC）")
}
