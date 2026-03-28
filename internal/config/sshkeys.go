package config

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// KeyMeta describes a stored SSH key pair.
type KeyMeta struct {
	Label       string `json:"label"`
	Algorithm   string `json:"algorithm"`
	DefaultUser string `json:"defaultUser"`
	PublicKey   string `json:"publicKey"`
}

func sshKeysDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "xman", "ssh-keys"), nil
}

func keyLabelDir(label string) (string, error) {
	base, err := sshKeysDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, label), nil
}

func privateKeyFilename(algorithm string) string {
	switch strings.ToLower(algorithm) {
	case "ed25519":
		return "id_ed25519"
	case "rsa-4096":
		return "id_rsa"
	case "ecdsa-256":
		return "id_ecdsa"
	default:
		return ""
	}
}

// CreateKeyPair generates a new SSH key pair and stores it on disk under
// the config dir at ssh-keys/<label>/.
func CreateKeyPair(label, algorithm, defaultUser string) (KeyMeta, error) {
	if label == "" {
		return KeyMeta{}, errors.New("label is required")
	}
	if strings.ContainsAny(label, `/\`+"\x00") || label == "." || label == ".." {
		return KeyMeta{}, errors.New("label contains invalid characters")
	}

	dir, err := keyLabelDir(label)
	if err != nil {
		return KeyMeta{}, err
	}
	if _, err := os.Stat(dir); err == nil {
		return KeyMeta{}, fmt.Errorf("key %q already exists", label)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return KeyMeta{}, err
	}

	var privPEM []byte
	var pubKey ssh.PublicKey
	var keyFileName string

	switch strings.ToLower(algorithm) {
	case "ed25519":
		keyFileName = "id_ed25519"
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			_ = os.RemoveAll(dir)
			return KeyMeta{}, err
		}
		der, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			_ = os.RemoveAll(dir)
			return KeyMeta{}, err
		}
		privPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		pubKey, err = ssh.NewPublicKey(pub)
		if err != nil {
			_ = os.RemoveAll(dir)
			return KeyMeta{}, err
		}

	case "rsa-4096":
		keyFileName = "id_rsa"
		priv, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			_ = os.RemoveAll(dir)
			return KeyMeta{}, err
		}
		privPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(priv),
		})
		pubKey, err = ssh.NewPublicKey(&priv.PublicKey)
		if err != nil {
			_ = os.RemoveAll(dir)
			return KeyMeta{}, err
		}

	case "ecdsa-256":
		keyFileName = "id_ecdsa"
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			_ = os.RemoveAll(dir)
			return KeyMeta{}, err
		}
		der, err := x509.MarshalECPrivateKey(priv)
		if err != nil {
			_ = os.RemoveAll(dir)
			return KeyMeta{}, err
		}
		privPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
		pubKey, err = ssh.NewPublicKey(&priv.PublicKey)
		if err != nil {
			_ = os.RemoveAll(dir)
			return KeyMeta{}, err
		}

	default:
		_ = os.RemoveAll(dir)
		return KeyMeta{}, fmt.Errorf("unsupported algorithm %q (supported: ed25519, rsa-4096, ecdsa-256)", algorithm)
	}

	privPath := filepath.Join(dir, keyFileName)
	pubPath := privPath + ".pub"

	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return KeyMeta{}, err
	}

	// Build authorized_keys-format public key line with an identifying comment.
	pubStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pubKey))) + " xman:" + label

	if err := os.WriteFile(pubPath, []byte(pubStr+"\n"), 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return KeyMeta{}, err
	}

	meta := KeyMeta{
		Label:       label,
		Algorithm:   algorithm,
		DefaultUser: defaultUser,
		PublicKey:   pubStr,
	}
	if metaJSON, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "meta.json"), metaJSON, 0o600)
	}

	return meta, nil
}

// ListKeys returns metadata for all stored key pairs, in directory order.
func ListKeys() ([]KeyMeta, error) {
	base, err := sshKeysDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(base)
	if errors.Is(err, os.ErrNotExist) {
		return []KeyMeta{}, nil
	}
	if err != nil {
		return nil, err
	}

	var keys []KeyMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(base, e.Name(), "meta.json"))
		if err != nil {
			continue
		}
		var m KeyMeta
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		keys = append(keys, m)
	}
	if keys == nil {
		keys = []KeyMeta{}
	}
	return keys, nil
}

// GetKey returns metadata for a single key by label.
func GetKey(label string) (KeyMeta, error) {
	dir, err := keyLabelDir(label)
	if err != nil {
		return KeyMeta{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return KeyMeta{}, fmt.Errorf("key %q not found", label)
	}
	var m KeyMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return KeyMeta{}, err
	}
	return m, nil
}

// LoadKeySigner loads the stored private key for label and returns its metadata
// plus an ssh.Signer for public-key auth.
func LoadKeySigner(label string) (KeyMeta, ssh.Signer, error) {
	meta, err := GetKey(label)
	if err != nil {
		return KeyMeta{}, nil, err
	}

	dir, err := keyLabelDir(label)
	if err != nil {
		return KeyMeta{}, nil, err
	}

	filename := privateKeyFilename(meta.Algorithm)
	if filename == "" {
		return KeyMeta{}, nil, fmt.Errorf("key %q uses unsupported algorithm %q", label, meta.Algorithm)
	}

	privPEM, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		return KeyMeta{}, nil, fmt.Errorf("reading private key for %q: %w", label, err)
	}

	signer, err := ssh.ParsePrivateKey(privPEM)
	if err != nil {
		return KeyMeta{}, nil, fmt.Errorf("parsing private key for %q: %w", label, err)
	}

	return meta, signer, nil
}

// UpdateKeyDefaultUser rewrites the stored metadata for label with a new
// default SSH username. An empty username is allowed.
func UpdateKeyDefaultUser(label, defaultUser string) (KeyMeta, error) {
	meta, err := GetKey(label)
	if err != nil {
		return KeyMeta{}, err
	}

	meta.DefaultUser = strings.TrimSpace(defaultUser)

	dir, err := keyLabelDir(label)
	if err != nil {
		return KeyMeta{}, err
	}

	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return KeyMeta{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), metaJSON, 0o600); err != nil {
		return KeyMeta{}, err
	}

	return meta, nil
}

// DeleteKey removes a key pair and its directory from disk.
func DeleteKey(label string) error {
	dir, err := keyLabelDir(label)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("key %q not found", label)
	}
	return os.RemoveAll(dir)
}
