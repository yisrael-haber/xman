import { useState, useEffect, FormEvent } from 'react';
import {
    Connect,
    LoadConnectionSettings,
    SaveConnectionSettings,
    ClearConnectionSettings,
} from '../../wailsjs/go/main/App';

interface Props {
    onConnected: (host: string) => void;
}

export default function LoginView({ onConnected }: Props) {
    const [url, setUrl]           = useState('https://');
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [insecure, setInsecure] = useState(false);
    const [remember, setRemember] = useState(false);
    const [loading, setLoading]   = useState(false);
    const [error, setError]       = useState('');

    // Pre-populate fields from saved settings on mount.
    // Password comes from the OS keyring (if available), never a file.
    useEffect(() => {
        LoadConnectionSettings().then(s => {
            if (s.url) {
                setUrl(s.url);
                setUsername(s.username ?? '');
                setInsecure(s.insecure ?? false);
                setRemember(true);
            }
            if (s.password) {
                setPassword(s.password);
            }
        });
    }, []);

    async function handleRememberToggle(checked: boolean) {
        setRemember(checked);
        if (!checked && username) {
            // User explicitly turned off "Remember" — clear stored settings
            await ClearConnectionSettings(username);
        }
    }

    async function handleSubmit(e: FormEvent) {
        e.preventDefault();
        setError('');
        setLoading(true);
        try {
            await Connect(url, username, password, insecure);
        } catch (err: any) {
            setError(String(err));
            setLoading(false);
            return;
        }

        // Save settings after a confirmed successful connect.
        // Errors here (e.g. keyring unavailable) are non-fatal — login still proceeds.
        if (remember) {
            try {
                await SaveConnectionSettings(url, username, password, insecure);
            } catch {
                // intentionally ignored
            }
        }

        setLoading(false);
        onConnected(new URL(url).host);
    }

    return (
        <div className="login-backdrop">
            <div className="login-card">
                <h1 className="login-title">manosphere</h1>
                <p className="login-subtitle">Connect to vCenter</p>

                <form onSubmit={handleSubmit} className="login-form">
                    <div className="field">
                        <label htmlFor="url">vCenter URL</label>
                        <input
                            id="url"
                            type="url"
                            value={url}
                            onChange={e => setUrl(e.target.value)}
                            placeholder="https://vcenter.example.com"
                            required
                            autoFocus
                        />
                    </div>

                    <div className="field">
                        <label htmlFor="username">Username</label>
                        <input
                            id="username"
                            type="text"
                            value={username}
                            onChange={e => setUsername(e.target.value)}
                            placeholder="administrator@vsphere.local"
                            required
                            autoComplete="username"
                        />
                    </div>

                    <div className="field">
                        <label htmlFor="password">Password</label>
                        <input
                            id="password"
                            type="password"
                            value={password}
                            onChange={e => setPassword(e.target.value)}
                            required
                            autoComplete="current-password"
                        />
                    </div>

                    <div className="checkbox-group">
                        <label className="checkbox-row">
                            <input
                                type="checkbox"
                                checked={insecure}
                                onChange={e => setInsecure(e.target.checked)}
                            />
                            <span>Skip TLS verification (self-signed certificate)</span>
                        </label>

                        <label className="checkbox-row">
                            <input
                                type="checkbox"
                                checked={remember}
                                onChange={e => handleRememberToggle(e.target.checked)}
                            />
                            <span>Remember settings</span>
                        </label>
                    </div>

                    {error && <p className="login-error">{error}</p>}

                    <button type="submit" className="login-btn" disabled={loading}>
                        {loading ? 'Connecting...' : 'Connect'}
                    </button>
                </form>
            </div>
        </div>
    );
}
