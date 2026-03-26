import { useState, useEffect, FormEvent } from 'react';
import {
    Connect,
    LoadConnectionSettings,
    SaveConnectionSettings,
    ClearConnectionSettings,
} from '../../wailsjs/go/main/App';
import { config } from '../../wailsjs/go/models';

interface Props {
    onConnected: (info: config.ConnectionInfo) => void;
}

export default function LoginView({ onConnected }: Props) {
    const [backendType, setBackendType] = useState<'vcenter' | 'workstation'>('vcenter');
    const [url, setUrl]           = useState('https://');
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [insecure, setInsecure] = useState(false);
    const [vmDir, setVmDir]       = useState('');
    const [remember, setRemember] = useState(true);
    const [loading, setLoading]   = useState(false);
    const [error, setError]       = useState('');

    // Pre-populate fields from saved settings on mount.
    useEffect(() => {
        LoadConnectionSettings().then(s => {
            if (!s.backendType) return;
            setBackendType(s.backendType as 'vcenter' | 'workstation');
            if (s.backendType === 'vcenter' && s.url) {
                setUrl(s.url);
                setUsername(s.username ?? '');
                setInsecure(s.insecure ?? false);
                setRemember(true);
                if (s.password) setPassword(s.password);
            } else if (s.backendType === 'workstation') {
                if (s.vmDir) setVmDir(s.vmDir);
                setRemember(true);
            }
        });
    }, []);

    async function handleRememberToggle(checked: boolean) {
        setRemember(checked);
        if (!checked) {
            await ClearConnectionSettings();
        }
    }

    async function handleSubmit(e: FormEvent) {
        e.preventDefault();
        setError('');
        setLoading(true);

        const req: config.ConnectRequest = config.ConnectRequest.createFrom({
            backendType,
            url,
            username,
            password,
            insecure,
            vmrunPath: '',
            vmDir,
        });

        let info: config.ConnectionInfo;
        try {
            info = await Connect(req);
        } catch (err: any) {
            setError(String(err));
            setLoading(false);
            return;
        }

        if (remember) {
            try {
                await SaveConnectionSettings(req);
            } catch {
                // intentionally ignored
            }
        }

        setLoading(false);
        onConnected(info);
    }

    return (
        <div className="login-backdrop">
            <div className="login-card">
                <h1 className="login-title">xman</h1>

                <div className="tab-bar" style={{margin: '0 -2.5rem 0.5rem', padding: '0 2.5rem', justifyContent: 'center'}}>
                    <button
                        type="button"
                        className={`tab ${backendType === 'vcenter' ? 'tab--active' : ''}`}
                        onClick={() => setBackendType('vcenter')}
                    >
                        vCenter
                    </button>
                    <button
                        type="button"
                        className={`tab ${backendType === 'workstation' ? 'tab--active' : ''}`}
                        onClick={() => setBackendType('workstation')}
                    >
                        Workstation
                    </button>
                </div>

                <form onSubmit={handleSubmit} className="login-form">
                    {backendType === 'vcenter' && <>
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
                                <span> Ignore self-signed certificate </span>
                            </label>
                        </div>
                    </>}

                    {backendType === 'workstation' && (
                        <div className="field">
                            <label htmlFor="vmDir">VM folder (optional)</label>
                            <input
                                id="vmDir"
                                type="text"
                                value={vmDir}
                                onChange={e => setVmDir(e.target.value)}
                                placeholder="Leave blank to use default location"
                                autoFocus
                            />
                        </div>
                    )}

                    <div className="checkbox-group">
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
