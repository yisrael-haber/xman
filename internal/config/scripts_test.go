package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestListScriptsCreatesDirectoryAndReturnsSortedScripts(t *testing.T) {
	setTestConfigHome(t)

	dir, err := scriptsDir()
	if err != nil {
		t.Fatalf("scriptsDir() error = %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	for name, content := range map[string]string{
		"deploy.sh":   "echo deploy\n",
		"cleanup.bat": "@echo off\r\necho cleanup\r\n",
		"notes.txt":   "hostname\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".ignored"), []byte("ignore"), 0o600); err != nil {
		t.Fatalf("WriteFile(hidden) error = %v", err)
	}

	catalog, err := ListScripts()
	if err != nil {
		t.Fatalf("ListScripts() error = %v", err)
	}
	if catalog.Directory != dir {
		t.Fatalf("Directory = %q, want %q", catalog.Directory, dir)
	}
	if len(catalog.Scripts) != 3 {
		t.Fatalf("Scripts len = %d, want %d (%+v)", len(catalog.Scripts), 3, catalog.Scripts)
	}
	if catalog.Scripts[0].Filename != "cleanup.bat" || catalog.Scripts[0].Kind != "windows-batch" {
		t.Fatalf("first script = %+v, want cleanup.bat windows-batch", catalog.Scripts[0])
	}
	if catalog.Scripts[1].Filename != "deploy.sh" || catalog.Scripts[1].Kind != "posix" {
		t.Fatalf("second script = %+v, want deploy.sh posix", catalog.Scripts[1])
	}
	if catalog.Scripts[2].Filename != "notes.txt" || catalog.Scripts[2].Kind != "generic" {
		t.Fatalf("third script = %+v, want notes.txt generic", catalog.Scripts[2])
	}
}

func TestLoadScriptReturnsContentAndRejectsTraversal(t *testing.T) {
	setTestConfigHome(t)

	dir, err := scriptsDir()
	if err != nil {
		t.Fatalf("scriptsDir() error = %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "deploy.sh"), []byte("#!/bin/sh\necho ready\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(deploy.sh) error = %v", err)
	}

	script, err := LoadScript("deploy.sh")
	if err != nil {
		t.Fatalf("LoadScript() error = %v", err)
	}
	if script.Name != "deploy" || script.Kind != "posix" {
		t.Fatalf("script metadata = %+v, want deploy/posix", script.ScriptInfo)
	}
	if script.Content != "#!/bin/sh\necho ready\n" {
		t.Fatalf("Content = %q, want original file contents", script.Content)
	}

	if _, err := LoadScript("../secrets.sh"); err == nil {
		t.Fatal("LoadScript(../secrets.sh) error = nil, want validation error")
	}
	if _, err := LoadScript(".hidden.sh"); err == nil {
		t.Fatal("LoadScript(.hidden.sh) error = nil, want validation error")
	}
}

func TestSaveScriptCreatesUpdatesRenamesAndDeletes(t *testing.T) {
	setTestConfigHome(t)

	saved, err := SaveScript(ScriptSaveRequest{
		Filename: "deploy.sh",
		Content:  "#!/bin/sh\necho deploy\n",
	})
	if err != nil {
		t.Fatalf("SaveScript(create) error = %v", err)
	}
	if saved.ID != "deploy.sh" || saved.Kind != "posix" {
		t.Fatalf("SaveScript(create) = %+v, want deploy.sh/posix", saved.ScriptInfo)
	}

	updated, err := SaveScript(ScriptSaveRequest{
		CurrentID: "deploy.sh",
		Filename:  "deploy.sh",
		Content:   "#!/bin/sh\necho updated\n",
	})
	if err != nil {
		t.Fatalf("SaveScript(update) error = %v", err)
	}
	if updated.Content != "#!/bin/sh\necho updated\n" {
		t.Fatalf("SaveScript(update) content = %q, want updated script", updated.Content)
	}

	renamed, err := SaveScript(ScriptSaveRequest{
		CurrentID: "deploy.sh",
		Filename:  "deploy-prod.sh",
		Content:   "#!/bin/sh\necho prod\n",
	})
	if err != nil {
		t.Fatalf("SaveScript(rename) error = %v", err)
	}
	if renamed.ID != "deploy-prod.sh" {
		t.Fatalf("SaveScript(rename) id = %q, want deploy-prod.sh", renamed.ID)
	}
	if _, err := LoadScript("deploy.sh"); err == nil {
		t.Fatal("LoadScript(old name) error = nil, want not found after rename")
	}

	if err := DeleteScript("deploy-prod.sh"); err != nil {
		t.Fatalf("DeleteScript() error = %v", err)
	}
	if _, err := LoadScript("deploy-prod.sh"); err == nil {
		t.Fatal("LoadScript(deleted) error = nil, want not found")
	}
}

func TestSaveScriptRejectsConflictsAndHiddenNames(t *testing.T) {
	setTestConfigHome(t)

	if _, err := SaveScript(ScriptSaveRequest{
		Filename: "first.sh",
		Content:  "echo first\n",
	}); err != nil {
		t.Fatalf("SaveScript(initial) error = %v", err)
	}

	if _, err := SaveScript(ScriptSaveRequest{
		Filename: "first.sh",
		Content:  "echo duplicate\n",
	}); err == nil {
		t.Fatal("SaveScript(duplicate) error = nil, want conflict")
	}

	if _, err := SaveScript(ScriptSaveRequest{
		Filename: ".hidden.sh",
		Content:  "echo hidden\n",
	}); err == nil {
		t.Fatal("SaveScript(hidden) error = nil, want validation error")
	}
}

func setTestConfigHome(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", dir)
		t.Setenv("APPDATA", dir)
	case "darwin":
		t.Setenv("HOME", dir)
	default:
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("HOME", dir)
	}
}
