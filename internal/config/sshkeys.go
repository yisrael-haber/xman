package config

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// KeyMeta describes a stored SSH key pair.
type KeyMeta struct {
	Label       string `json:"label"`
	Algorithm   string `json:"algorithm"`
	DefaultUser string `json:"defaultUser"`
	PublicKey   string `json:"publicKey"`
}

type signerCacheEntry struct {
	path    string
	modTime time.Time
	size    int64
	signer  ssh.Signer
}

var signerCache struct {
	mu      sync.RWMutex
	entries map[string]signerCacheEntry
}

func sshKeysDir() (string, error) {
	dir, err := appConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ssh-keys"), nil
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

func marshalOpenSSHPrivateKey(key any, comment string) ([]byte, error) {
	block, err := ssh.MarshalPrivateKey(key, comment)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(block), nil
}

func loadNativeCompatiblePrivateKey(path, label string) ([]byte, error) {
	privBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if bytes.Contains(privBytes, []byte("BEGIN OPENSSH PRIVATE KEY")) {
		return privBytes, nil
	}

	rawKey, err := ssh.ParseRawPrivateKey(privBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing legacy private key for %q: %w", label, err)
	}

	rewritten, err := marshalOpenSSHPrivateKey(rawKey, "xman:"+label)
	if err != nil {
		return nil, fmt.Errorf("rewriting private key for %q to OpenSSH format: %w", label, err)
	}

	if err := os.WriteFile(path, rewritten, 0o600); err != nil {
		return nil, fmt.Errorf("writing upgraded private key for %q: %w", label, err)
	}

	return rewritten, nil
}

func ensureNativeCompatiblePrivateKey(path, label string) error {
	_, err := loadNativeCompatiblePrivateKey(path, label)
	return err
}

func cachedSigner(label, path string, info os.FileInfo) (ssh.Signer, bool) {
	signerCache.mu.RLock()
	defer signerCache.mu.RUnlock()

	entry, ok := signerCache.entries[label]
	if !ok {
		return nil, false
	}
	if entry.path != path || entry.modTime != info.ModTime() || entry.size != info.Size() {
		return nil, false
	}
	return entry.signer, true
}

func storeCachedSigner(label, path string, info os.FileInfo, signer ssh.Signer) {
	signerCache.mu.Lock()
	defer signerCache.mu.Unlock()

	if signerCache.entries == nil {
		signerCache.entries = make(map[string]signerCacheEntry)
	}
	signerCache.entries[label] = signerCacheEntry{
		path:    path,
		modTime: info.ModTime(),
		size:    info.Size(),
		signer:  signer,
	}
}

func invalidateCachedSigner(label string) {
	signerCache.mu.Lock()
	defer signerCache.mu.Unlock()
	delete(signerCache.entries, label)
}

func privateKeyInfo(meta KeyMeta) (string, os.FileInfo, error) {
	dir, err := keyLabelDir(meta.Label)
	if err != nil {
		return "", nil, err
	}

	filename := privateKeyFilename(meta.Algorithm)
	if filename == "" {
		return "", nil, fmt.Errorf("key %q uses unsupported algorithm %q", meta.Label, meta.Algorithm)
	}

	path := filepath.Join(dir, filename)
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, fmt.Errorf("private key for %q not found: %w", meta.Label, err)
	}
	return path, info, nil
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
		privPEM, err = marshalOpenSSHPrivateKey(priv, "xman:"+label)
		if err != nil {
			_ = os.RemoveAll(dir)
			return KeyMeta{}, err
		}
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
		privPEM, err = marshalOpenSSHPrivateKey(priv, "xman:"+label)
		if err != nil {
			_ = os.RemoveAll(dir)
			return KeyMeta{}, err
		}
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
		privPEM, err = marshalOpenSSHPrivateKey(priv, "xman:"+label)
		if err != nil {
			_ = os.RemoveAll(dir)
			return KeyMeta{}, err
		}
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

	invalidateCachedSigner(meta.Label)
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

	path, info, err := privateKeyInfo(meta)
	if err != nil {
		return KeyMeta{}, nil, err
	}

	if signer, ok := cachedSigner(label, path, info); ok {
		return meta, signer, nil
	}

	privPEM, err := loadNativeCompatiblePrivateKey(path, label)
	if err != nil {
		return KeyMeta{}, nil, err
	}
	signer, err := ssh.ParsePrivateKey(privPEM)
	if err != nil {
		return KeyMeta{}, nil, fmt.Errorf("parsing private key for %q: %w", label, err)
	}

	if refreshedInfo, statErr := os.Stat(path); statErr == nil {
		info = refreshedInfo
	}
	storeCachedSigner(label, path, info, signer)
	return meta, signer, nil
}

// PrivateKeyPath returns the stored private key path for label, along with its
// metadata. This is used by external/native SSH launch flows that need the key
// file on disk rather than an in-memory signer.
func PrivateKeyPath(label string) (KeyMeta, string, error) {
	meta, err := GetKey(label)
	if err != nil {
		return KeyMeta{}, "", err
	}

	path, _, err := privateKeyInfo(meta)
	if err != nil {
		return KeyMeta{}, "", err
	}

	if err := ensureNativeCompatiblePrivateKey(path, label); err != nil {
		return KeyMeta{}, "", err
	}

	return meta, path, nil
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

	invalidateCachedSigner(label)
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
	invalidateCachedSigner(label)
	return os.RemoveAll(dir)
}
