package oidcserver

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"

	"github.com/UruhaLushia/headdeck/internal/securefile"
	"github.com/go-jose/go-jose/v4"
)

type signingKey struct {
	private *rsa.PrivateKey
	id      string
}

func (k signingKey) SignatureAlgorithm() jose.SignatureAlgorithm { return jose.RS256 }
func (k signingKey) Algorithm() jose.SignatureAlgorithm          { return jose.RS256 }
func (k signingKey) ID() string                                  { return k.id }
func (k signingKey) Key() any                                    { return k.private }
func (k signingKey) Use() string                                 { return "sig" }

type publicKey struct{ signingKey }

func ValidateKey(filename string) error { _, err := loadKey(filename); return err }

func (k publicKey) Key() any { return &k.private.PublicKey }

func loadKey(filename string) (signingKey, error) {
	if err := securefile.PrepareDirectory(filepath.Dir(filename), 0700); err != nil {
		return signingKey{}, err
	}
	info, err := os.Lstat(filename)
	if errors.Is(err, os.ErrNotExist) {
		key, e := rsa.GenerateKey(rand.Reader, 2048)
		if e != nil {
			return signingKey{}, e
		}
		file, e := os.CreateTemp(filepath.Dir(filename), ".oidc-key-*")
		if e != nil {
			return signingKey{}, e
		}
		defer os.Remove(file.Name())
		e = pem.Encode(file, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
		if e == nil {
			e = file.Sync()
		}
		closeErr := file.Close()
		if e != nil {
			return signingKey{}, e
		}
		if closeErr != nil {
			return signingKey{}, closeErr
		}
		if e = os.Link(file.Name(), filename); e != nil && !errors.Is(e, os.ErrExist) {
			return signingKey{}, e
		}
		info, err = os.Lstat(filename)
	}
	if err != nil {
		return signingKey{}, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 16384 || !securefile.Owned(info) {
		return signingKey{}, errors.New("OIDC signing key must be a regular file with mode 0600")
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return signingKey{}, err
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return signingKey{}, errors.New("invalid OIDC signing key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return signingKey{}, err
	}
	if key.N.BitLen() < 2048 {
		return signingKey{}, errors.New("OIDC signing key is too small")
	}
	digest := sha256.Sum256(x509.MarshalPKCS1PublicKey(&key.PublicKey))
	return signingKey{private: key, id: hex.EncodeToString(digest[:16])}, nil
}
