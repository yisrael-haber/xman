package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxStoredScriptBytes = 1 << 20 // 1 MiB

type ScriptInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Kind     string `json:"kind"`
}

type ScriptCatalog struct {
	Directory string       `json:"directory"`
	Scripts   []ScriptInfo `json:"scripts"`
}

type StoredScript struct {
	ScriptInfo
	Content string `json:"content"`
}

type ScriptSaveRequest struct {
	CurrentID string `json:"currentID"`
	Filename  string `json:"filename"`
	Content   string `json:"content"`
}

func scriptsDir() (string, error) {
	base, err := appConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "scripts"), nil
}

func ListScripts() (ScriptCatalog, error) {
	dir, err := scriptsDir()
	if err != nil {
		return ScriptCatalog{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ScriptCatalog{}, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ScriptCatalog{}, err
	}

	scripts := make([]ScriptInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}

		filename := entry.Name()
		ext := filepath.Ext(filename)
		name := strings.TrimSuffix(filename, ext)
		if strings.TrimSpace(name) == "" {
			name = filename
		}

		scripts = append(scripts, ScriptInfo{
			ID:       filename,
			Name:     name,
			Filename: filename,
			Kind:     detectScriptKind(filename),
		})
	}

	sort.Slice(scripts, func(i, j int) bool {
		if scripts[i].Name != scripts[j].Name {
			return scripts[i].Name < scripts[j].Name
		}
		return scripts[i].Filename < scripts[j].Filename
	})

	return ScriptCatalog{
		Directory: dir,
		Scripts:   scripts,
	}, nil
}

func LoadScript(id string) (StoredScript, error) {
	id, err := normalizeScriptID(id)
	if err != nil {
		return StoredScript{}, err
	}

	dir, err := scriptsDir()
	if err != nil {
		return StoredScript{}, err
	}

	path := filepath.Join(dir, id)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return StoredScript{}, fmt.Errorf("script %q not found", id)
	}
	if err != nil {
		return StoredScript{}, err
	}
	if !info.Mode().IsRegular() {
		return StoredScript{}, fmt.Errorf("script %q is not a regular file", id)
	}
	if info.Size() > maxStoredScriptBytes {
		return StoredScript{}, fmt.Errorf("script %q exceeds the 1 MiB size limit", id)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return StoredScript{}, err
	}

	ext := filepath.Ext(id)
	name := strings.TrimSuffix(id, ext)
	if strings.TrimSpace(name) == "" {
		name = id
	}

	return StoredScript{
		ScriptInfo: ScriptInfo{
			ID:       id,
			Name:     name,
			Filename: id,
			Kind:     detectScriptKind(id),
		},
		Content: string(content),
	}, nil
}

func SaveScript(req ScriptSaveRequest) (StoredScript, error) {
	currentID := strings.TrimSpace(req.CurrentID)
	if currentID != "" {
		var err error
		currentID, err = normalizeScriptID(currentID)
		if err != nil {
			return StoredScript{}, err
		}
	}

	filename, err := normalizeScriptID(req.Filename)
	if err != nil {
		return StoredScript{}, err
	}

	dir, err := scriptsDir()
	if err != nil {
		return StoredScript{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return StoredScript{}, err
	}

	if currentID != "" {
		currentPath := filepath.Join(dir, currentID)
		info, err := os.Stat(currentPath)
		if errors.Is(err, os.ErrNotExist) {
			return StoredScript{}, fmt.Errorf("script %q not found", currentID)
		}
		if err != nil {
			return StoredScript{}, err
		}
		if !info.Mode().IsRegular() {
			return StoredScript{}, fmt.Errorf("script %q is not a regular file", currentID)
		}
	}

	path := filepath.Join(dir, filename)
	if currentID == "" || currentID != filename {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return StoredScript{}, fmt.Errorf("script %q already exists", filename)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return StoredScript{}, err
		}
	}

	if len(req.Content) > maxStoredScriptBytes {
		return StoredScript{}, fmt.Errorf("script %q exceeds the 1 MiB size limit", filename)
	}
	if err := os.WriteFile(path, []byte(req.Content), 0o600); err != nil {
		return StoredScript{}, err
	}
	if currentID != "" && currentID != filename {
		if err := os.Remove(filepath.Join(dir, currentID)); err != nil {
			return StoredScript{}, err
		}
	}

	return LoadScript(filename)
}

func DeleteScript(id string) error {
	id, err := normalizeScriptID(id)
	if err != nil {
		return err
	}

	dir, err := scriptsDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, id)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("script %q not found", id)
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("script %q is not a regular file", id)
	}
	return os.Remove(path)
}

func normalizeScriptID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("script id is required")
	}
	if strings.HasPrefix(id, ".") {
		return "", errors.New("script id cannot start with a dot")
	}
	if id != filepath.Base(id) || strings.ContainsAny(id, `/\`+"\x00") || id == "." || id == ".." {
		return "", errors.New("script id contains invalid characters")
	}
	return id, nil
}

func detectScriptKind(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".sh", ".bash", ".zsh", ".ksh":
		return "posix"
	case ".cmd", ".bat":
		return "windows-batch"
	case ".ps1":
		return "powershell"
	default:
		return "generic"
	}
}
