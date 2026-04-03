import { useEffect, useState } from 'react';
import { config } from '../../../../wailsjs/go/models';
import { DeleteScript, GetScript, ListScripts, SaveScript } from '../../../../wailsjs/go/main/App';
import { scriptKindFromFilename, scriptKindLabel } from '../../../utils/scripts';

export default function StoredScriptsManager() {
    const [catalog, setCatalog] = useState<config.ScriptCatalog | null>(null);
    const [selectedId, setSelectedId] = useState('');
    const [currentScript, setCurrentScript] = useState<config.StoredScript | null>(null);
    const [filename, setFilename] = useState('');
    const [content, setContent] = useState('');
    const [loadingCatalog, setLoadingCatalog] = useState(false);
    const [loadingScript, setLoadingScript] = useState(false);
    const [saving, setSaving] = useState(false);
    const [deleting, setDeleting] = useState(false);
    const [error, setError] = useState('');
    const [status, setStatus] = useState('');

    const detectedKind = scriptKindFromFilename(filename);
    const dirty = currentScript
        ? currentScript.filename !== filename || currentScript.content !== content
        : filename.trim().length > 0 || content.length > 0;

    useEffect(() => {
        void refreshCatalog();
    }, []);

    useEffect(() => {
        if (!selectedId) {
            setCurrentScript(null);
            return;
        }

        setLoadingScript(true);
        setError('');
        setStatus('');
        void GetScript(selectedId)
            .then(script => {
                setCurrentScript(script);
                setFilename(script.filename);
                setContent(script.content);
            })
            .catch((nextError: unknown) => {
                setCurrentScript(null);
                setError(String(nextError));
            })
            .finally(() => setLoadingScript(false));
    }, [selectedId]);

    async function refreshCatalog(preferredId?: string, preserveEmptySelection: boolean = false) {
        setLoadingCatalog(true);
        setError('');
        try {
            const nextCatalog = await ListScripts();
            setCatalog(nextCatalog);

            const availableIDs = new Set((nextCatalog.scripts ?? []).map(script => script.id));
            let nextSelected = preferredId ?? selectedId;
            if (nextSelected && !availableIDs.has(nextSelected)) {
                nextSelected = '';
            }
            if (!nextSelected && !preserveEmptySelection) {
                nextSelected = nextCatalog.scripts[0]?.id ?? '';
            }
            setSelectedId(nextSelected);
            if (!nextSelected) {
                setCurrentScript(null);
            }
        } catch (nextError: unknown) {
            setCatalog(null);
            setSelectedId('');
            setCurrentScript(null);
            setError(String(nextError));
        } finally {
            setLoadingCatalog(false);
        }
    }

    function resetEditor() {
        setSelectedId('');
        setCurrentScript(null);
        setFilename('');
        setContent('');
        setError('');
        setStatus('');
    }

    function confirmDiscardIfNeeded(): boolean {
        if (!dirty) {
            return true;
        }
        return window.confirm('Discard unsaved script changes?');
    }

    function handleSelect(id: string) {
        if (id === selectedId) {
            return;
        }
        if (!confirmDiscardIfNeeded()) {
            return;
        }
        setSelectedId(id);
    }

    function handleNewScript() {
        if (!confirmDiscardIfNeeded()) {
            return;
        }
        resetEditor();
    }

    function handleResetDraft() {
        setError('');
        setStatus('');
        if (currentScript) {
            setFilename(currentScript.filename);
            setContent(currentScript.content);
            return;
        }
        setFilename('');
        setContent('');
    }

    async function handleSave() {
        setSaving(true);
        setError('');
        setStatus('');
        try {
            const saved = await SaveScript({
                currentID: currentScript?.id ?? '',
                filename: filename.trim(),
                content,
            });
            setCurrentScript(saved);
            setFilename(saved.filename);
            setContent(saved.content);
            setSelectedId(saved.id);
            setStatus(currentScript ? 'Script updated.' : 'Script saved.');
            await refreshCatalog(saved.id, true);
        } catch (nextError: unknown) {
            setError(String(nextError));
        } finally {
            setSaving(false);
        }
    }

    async function handleDelete() {
        if (!currentScript) {
            return;
        }
        if (!window.confirm(`Delete script "${currentScript.filename}"? This cannot be undone.`)) {
            return;
        }

        setDeleting(true);
        setError('');
        setStatus('');
        try {
            await DeleteScript(currentScript.id);
            resetEditor();
            setStatus('Script deleted.');
            await refreshCatalog('', true);
        } catch (nextError: unknown) {
            setError(String(nextError));
        } finally {
            setDeleting(false);
        }
    }

    const canSave = !saving && !loadingScript && filename.trim().length > 0 && dirty;

    return (
        <div className="ssh-keys-layout">
            <div className="ssh-keys-list">
                <div className="ssh-keys-list-header">Stored Scripts</div>
                {loadingCatalog && <p className="vm-browser-empty">Loading…</p>}
                {!loadingCatalog && !(catalog?.scripts?.length) && (
                    <p className="vm-browser-empty">No stored scripts yet.</p>
                )}
                {(catalog?.scripts ?? []).map(script => (
                    <div
                        key={script.id}
                        className={`ssh-key-item ${selectedId === script.id ? 'ssh-key-item--active' : ''}`}
                        onClick={() => handleSelect(script.id)}
                    >
                        <div className="ssh-key-item-label">{script.name}</div>
                        <div className="ssh-key-item-meta">
                            {script.filename} · {scriptKindLabel(script.kind)}
                        </div>
                    </div>
                ))}
                {error && <p className="form-error section-error">{error}</p>}
            </div>

            <div className="ssh-keys-detail">
                <div className="scripts-toolbar">
                    <div className="scripts-toolbar-copy">
                        <span className="exec-live-session-title">Stored Scripts</span>
                        <span className="exec-live-session-text">
                            Save reusable guest scripts here. They will be available from the VM Run tab.
                        </span>
                        <span className="scripts-directory">{catalog?.directory || 'Loading scripts directory…'}</span>
                    </div>
                    <div className="scripts-actions">
                        <button className="btn-secondary" onClick={() => void refreshCatalog(selectedId, !selectedId)} disabled={loadingCatalog}>
                            {loadingCatalog ? 'Refreshing…' : 'Refresh'}
                        </button>
                        <button className="btn-secondary" onClick={handleNewScript} disabled={saving || deleting}>
                            New Script
                        </button>
                    </div>
                </div>

                <div className="script-manager-shell">
                    <div className="ssh-key-detail-header">
                        <div>
                            <div className="ssh-key-detail-title">
                                {currentScript ? currentScript.name : 'Create Stored Script'}
                            </div>
                            <div className="ssh-key-detail-meta">
                                <span className="ssh-key-badge">{scriptKindLabel(detectedKind)}</span>
                                {currentScript && <span className="ssh-key-badge">{currentScript.filename}</span>}
                            </div>
                        </div>
                        {currentScript && (
                            <button className="btn-danger" onClick={() => void handleDelete()} disabled={deleting || saving}>
                                {deleting ? 'Deleting…' : 'Delete'}
                            </button>
                        )}
                    </div>

                    <div className="field">
                        <label>Filename</label>
                        <input
                            value={filename}
                            onChange={e => setFilename(e.target.value)}
                            placeholder="e.g. bootstrap.sh or bootstrap.cmd"
                            autoComplete="off"
                            disabled={loadingScript || saving || deleting}
                        />
                        <div className="field-help">
                            File extensions control guest compatibility: `.sh` for POSIX, `.cmd` or `.bat` for Windows, `.txt` for generic text.
                        </div>
                    </div>

                    <div className="field">
                        <label>Script Content</label>
                        <textarea
                            className="script-editor"
                            value={content}
                            onChange={e => setContent(e.target.value)}
                            placeholder="Enter the script body exactly as it should run in the guest"
                            disabled={loadingScript || saving || deleting}
                            rows={16}
                        />
                    </div>

                    {status && <p className="script-status">{status}</p>}

                    <div className="script-editor-actions">
                        <button className="btn-primary" onClick={() => void handleSave()} disabled={!canSave}>
                            {saving ? 'Saving…' : currentScript ? 'Save Changes' : 'Save Script'}
                        </button>
                        {(currentScript || dirty) && (
                            <button className="btn-secondary" onClick={handleResetDraft} disabled={saving || deleting}>
                                {currentScript ? 'Discard Changes' : 'Clear'}
                            </button>
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
}
