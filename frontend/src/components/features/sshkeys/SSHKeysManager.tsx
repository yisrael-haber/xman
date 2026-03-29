import { useState, useEffect } from 'react';
import { config } from '../../../../wailsjs/go/models';
import { CreateSSHKey, ListSSHKeys, DeleteSSHKey } from '../../../../wailsjs/go/main/App';

const ALGORITHMS = ['ed25519', 'rsa-4096', 'ecdsa-256'];

export default function SSHKeysManager() {
    const [keys,        setKeys]        = useState<config.KeyMeta[]>([]);
    const [selected,    setSelected]    = useState<config.KeyMeta | null>(null);
    const [loading,     setLoading]     = useState(false);
    const [error,       setError]       = useState('');
    const [copied,      setCopied]      = useState(false);

    const [label,       setLabel]       = useState('');
    const [algorithm,   setAlgorithm]   = useState('ed25519');
    const [defaultUser, setDefaultUser] = useState('');
    const [creating,    setCreating]    = useState(false);
    const [createError, setCreateError] = useState('');

    async function loadKeys() {
        setLoading(true);
        setError('');
        try {
            const list = await ListSSHKeys();
            setKeys(list ?? []);
            setSelected(prev => prev ? (list.find(k => k.label === prev.label) ?? null) : null);
        } catch (e: any) {
            setError(String(e));
        } finally {
            setLoading(false);
        }
    }

    useEffect(() => { loadKeys(); }, []);

    async function handleCreate() {
        setCreateError('');
        setCreating(true);
        try {
            const meta = await CreateSSHKey(label.trim(), algorithm, defaultUser.trim());
            setKeys(prev => [...prev, meta]);
            setSelected(meta);
            setLabel('');
            setDefaultUser('');
            setCopied(false);
        } catch (e: any) {
            setCreateError(String(e));
        } finally {
            setCreating(false);
        }
    }

    async function handleDelete(k: config.KeyMeta) {
        if (!window.confirm(`Delete key "${k.label}"? This cannot be undone.`)) return;
        setError('');
        try {
            await DeleteSSHKey(k.label);
            setKeys(prev => prev.filter(x => x.label !== k.label));
            if (selected?.label === k.label) setSelected(null);
        } catch (e: any) {
            setError(String(e));
        }
    }

    function handleCopy() {
        if (!selected) return;
        navigator.clipboard.writeText(selected.publicKey).then(() => {
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
        });
    }

    const canCreate = !creating && label.trim().length > 0;

    return (
        <div className="ssh-keys-layout">
            <div className="ssh-keys-list">
                <div className="ssh-keys-list-header">Stored Keys</div>
                {loading && <p className="vm-browser-empty">Loading…</p>}
                {!loading && keys.length === 0 && (
                    <p className="vm-browser-empty">No keys yet.</p>
                )}
                {keys.map(k => (
                    <div
                        key={k.label}
                        className={`ssh-key-item ${selected?.label === k.label ? 'ssh-key-item--active' : ''}`}
                        onClick={() => { setSelected(k); setCopied(false); }}
                    >
                        <div className="ssh-key-item-label">{k.label}</div>
                        <div className="ssh-key-item-meta">
                            {k.algorithm}{k.defaultUser ? ` · ${k.defaultUser}` : ''}
                        </div>
                    </div>
                ))}
                {error && <p className="form-error" style={{ margin: '0.5rem' }}>{error}</p>}
            </div>

            <div className="ssh-keys-detail">
                {selected && (
                    <div className="ssh-key-detail-view">
                        <div className="ssh-key-detail-header">
                            <span className="ssh-key-detail-title">{selected.label}</span>
                            <button className="btn-danger" onClick={() => handleDelete(selected)}>
                                Delete
                            </button>
                        </div>

                        <div className="ssh-key-detail-meta">
                            <span className="ssh-key-badge">{selected.algorithm}</span>
                            {selected.defaultUser && (
                                <span className="ssh-key-badge">user: {selected.defaultUser}</span>
                            )}
                        </div>

                        <div className="field" style={{ marginTop: '0.75rem' }}>
                            <label>Public Key</label>
                            <textarea
                                className="ssh-key-pubkey"
                                readOnly
                                value={selected.publicKey}
                                rows={3}
                            />
                            <button
                                className="btn-secondary"
                                style={{ marginTop: '0.4rem' }}
                                onClick={handleCopy}
                            >
                                {copied ? 'Copied!' : 'Copy to clipboard'}
                            </button>
                        </div>
                    </div>
                )}

                <div
                    className="ssh-key-create-form"
                    style={selected ? { borderTop: '1px solid #1e3044', paddingTop: '1.25rem', marginTop: '1.25rem' } : {}}
                >
                    <div className="ssh-key-detail-title" style={{ marginBottom: '1rem' }}>
                        {selected ? 'Create Another Key' : 'Create New Key Pair'}
                    </div>

                    <div className="field">
                        <label>Label</label>
                        <input
                            value={label}
                            onChange={e => setLabel(e.target.value)}
                            placeholder="e.g. lab-access"
                            autoComplete="off"
                        />
                    </div>

                    <div className="field">
                        <label>Algorithm</label>
                        <select value={algorithm} onChange={e => setAlgorithm(e.target.value)}>
                            {ALGORITHMS.map(a => (
                                <option key={a} value={a}>{a}</option>
                            ))}
                        </select>
                    </div>

                    <div className="field">
                        <label>
                            Default Username{' '}
                            <span style={{ fontWeight: 400, opacity: 0.6 }}>(optional — pre-fills deploy form)</span>
                        </label>
                        <input
                            value={defaultUser}
                            onChange={e => setDefaultUser(e.target.value)}
                            placeholder="e.g. root"
                            autoComplete="off"
                        />
                    </div>

                    {createError && <p className="form-error">{createError}</p>}

                    <button className="btn-primary" onClick={handleCreate} disabled={!canCreate}>
                        {creating ? 'Generating…' : 'Generate Key Pair'}
                    </button>
                </div>
            </div>
        </div>
    );
}
