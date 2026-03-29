import { useEffect, useState } from 'react';
import { config } from '../../../../wailsjs/go/models';
import {
    CreateGuestCredential,
    DeleteGuestCredential,
    GetGuestCredential,
    ListGuestCredentials,
    UpdateGuestCredential,
} from '../../../../wailsjs/go/main/App';

export default function GuestCredentialsManager() {
    const [credentials, setCredentials] = useState<config.GuestCredentialMeta[]>([]);
    const [selected, setSelected] = useState<config.GuestCredentialMeta | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const [details, setDetails] = useState<config.GuestCredential | null>(null);
    const [loadingDetails, setLoadingDetails] = useState(false);
    const [showPassword, setShowPassword] = useState(false);
    const [editingLabel, setEditingLabel] = useState('');

    const [label, setLabel] = useState('');
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [creating, setCreating] = useState(false);
    const [createError, setCreateError] = useState('');
    const [savingEdit, setSavingEdit] = useState(false);

    async function loadCredentials() {
        setLoading(true);
        setError('');
        try {
            const list = await ListGuestCredentials();
            setCredentials(list ?? []);
            setSelected(prev => prev ? (list.find(c => c.label === prev.label) ?? null) : null);
        } catch (e: any) {
            setError(String(e));
        } finally {
            setLoading(false);
        }
    }

    useEffect(() => { loadCredentials(); }, []);

    useEffect(() => {
        setDetails(null);
        setShowPassword(false);
        setEditingLabel('');
        setCreateError('');
        setLabel('');
        setUsername('');
        setPassword('');
    }, [selected?.label]);

    async function ensureDetails(labelToLoad: string) {
        if (details?.label === labelToLoad) return details;

        setLoadingDetails(true);
        try {
            const full = await GetGuestCredential(labelToLoad);
            setDetails(full);
            return full;
        } finally {
            setLoadingDetails(false);
        }
    }

    async function handleCreate() {
        setCreateError('');
        setCreating(true);
        try {
            const meta = await CreateGuestCredential(label.trim(), username.trim(), password);
            setCredentials(prev => [...prev, meta]);
            setSelected(meta);
            setLabel('');
            setUsername('');
            setPassword('');
        } catch (e: any) {
            setCreateError(String(e));
        } finally {
            setCreating(false);
        }
    }

    async function handleDelete(credential: config.GuestCredentialMeta) {
        if (!window.confirm(`Delete credential "${credential.label}"? This cannot be undone.`)) return;
        setError('');
        try {
            await DeleteGuestCredential(credential.label);
            setCredentials(prev => prev.filter(c => c.label !== credential.label));
            if (selected?.label === credential.label) setSelected(null);
        } catch (e: any) {
            setError(String(e));
        }
    }

    async function handleTogglePassword() {
        if (!selected) return;
        if (showPassword) {
            setShowPassword(false);
            return;
        }
        setError('');
        try {
            await ensureDetails(selected.label);
            setShowPassword(true);
        } catch (e: any) {
            setError(String(e));
        }
    }

    async function handleStartEdit() {
        if (!selected) return;
        setError('');
        setCreateError('');
        try {
            const full = await ensureDetails(selected.label);
            setEditingLabel(selected.label);
            setLabel(full.label);
            setUsername(full.username);
            setPassword(full.password);
            setShowPassword(false);
        } catch (e: any) {
            setError(String(e));
        }
    }

    function handleCancelEdit() {
        setEditingLabel('');
        setCreateError('');
        setLabel('');
        setUsername('');
        setPassword('');
    }

    async function handleSaveEdit() {
        if (!editingLabel) return;
        setCreateError('');
        setSavingEdit(true);
        try {
            const updated = await UpdateGuestCredential(editingLabel, label.trim(), username.trim(), password);
            setCredentials(prev => prev.map(c => c.label === editingLabel ? updated : c));
            setSelected(updated);
            setDetails(config.GuestCredential.createFrom({
                label: updated.label,
                username: updated.username,
                password,
            }));
            setEditingLabel('');
            setLabel('');
            setUsername('');
            setPassword('');
            setShowPassword(false);
        } catch (e: any) {
            setCreateError(String(e));
        } finally {
            setSavingEdit(false);
        }
    }

    const canCreate = !creating && label.trim().length > 0 && username.trim().length > 0 && password.length > 0;
    const isEditing = editingLabel.length > 0;
    const canSaveEdit = !savingEdit && label.trim().length > 0 && username.trim().length > 0 && password.length > 0;

    return (
        <div className="ssh-keys-layout">
            <div className="ssh-keys-list">
                <div className="ssh-keys-list-header">Stored Credentials</div>
                {loading && <p className="vm-browser-empty">Loading…</p>}
                {!loading && credentials.length === 0 && (
                    <p className="vm-browser-empty">No guest credentials yet.</p>
                )}
                {credentials.map(credential => (
                    <div
                        key={credential.label}
                        className={`ssh-key-item ${selected?.label === credential.label ? 'ssh-key-item--active' : ''}`}
                        onClick={() => setSelected(credential)}
                    >
                        <div className="ssh-key-item-label">{credential.label}</div>
                        <div className="ssh-key-item-meta">{credential.username}</div>
                    </div>
                ))}
                {error && <p className="form-error" style={{ margin: '0.5rem' }}>{error}</p>}
            </div>

            <div className="ssh-keys-detail">
                {selected && (
                    <div className="ssh-key-detail-view">
                        <div className="ssh-key-detail-header">
                            <span className="ssh-key-detail-title">{selected.label}</span>
                            <div style={{ display: 'flex', gap: '0.5rem' }}>
                                <button className="btn-secondary" onClick={() => void handleStartEdit()}>
                                    Edit
                                </button>
                                <button className="btn-danger" onClick={() => handleDelete(selected)}>
                                    Delete
                                </button>
                            </div>
                        </div>

                        <div className="ssh-key-detail-meta">
                            <span className="ssh-key-badge">user: {selected.username}</span>
                        </div>

                        <div className="field" style={{ marginTop: '0.75rem' }}>
                            <label>Password</label>
                            <div className="input-with-btn">
                                <input
                                    type={showPassword ? 'text' : 'password'}
                                    readOnly
                                    value={showPassword ? (details?.password ?? '') : '••••••••'}
                                    placeholder={loadingDetails ? 'Loading…' : 'Hidden'}
                                />
                                <button className="btn-secondary" onClick={() => void handleTogglePassword()} disabled={loadingDetails}>
                                    {showPassword ? 'Hide' : 'View'}
                                </button>
                            </div>
                        </div>

                        <div className="info-inline-note">
                            Passwords stay hidden unless you explicitly reveal or edit a selected credential.
                        </div>
                    </div>
                )}

                <div
                    className="ssh-key-create-form"
                    style={selected ? { borderTop: '1px solid #1e3044', paddingTop: '1.25rem', marginTop: '1.25rem' } : {}}
                >
                    <div className="ssh-key-detail-title" style={{ marginBottom: '1rem' }}>
                        {isEditing ? 'Edit Guest Credential' : selected ? 'Create Another Credential' : 'Create Guest Credential'}
                    </div>

                    <div className="field">
                        <label>Label</label>
                        <input
                            value={label}
                            onChange={e => setLabel(e.target.value)}
                            placeholder="e.g. ubuntu-admin"
                            autoComplete="off"
                        />
                    </div>

                    <div className="field">
                        <label>Username</label>
                        <input
                            value={username}
                            onChange={e => setUsername(e.target.value)}
                            placeholder="e.g. root"
                            autoComplete="off"
                        />
                    </div>

                    <div className="field">
                        <label>Password</label>
                        <input
                            type="password"
                            value={password}
                            onChange={e => setPassword(e.target.value)}
                            autoComplete="off"
                        />
                    </div>

                    {createError && <p className="form-error">{createError}</p>}

                    {isEditing ? (
                        <div style={{ display: 'flex', gap: '0.5rem' }}>
                            <button className="btn-primary" onClick={() => void handleSaveEdit()} disabled={!canSaveEdit}>
                                {savingEdit ? 'Saving…' : 'Save Changes'}
                            </button>
                            <button className="btn-secondary" onClick={handleCancelEdit} disabled={savingEdit}>
                                Cancel
                            </button>
                        </div>
                    ) : (
                        <button className="btn-primary" onClick={handleCreate} disabled={!canCreate}>
                            {creating ? 'Saving…' : 'Save Credential'}
                        </button>
                    )}
                </div>
            </div>
        </div>
    );
}
