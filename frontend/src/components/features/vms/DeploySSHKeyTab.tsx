import { useState, useEffect } from 'react';
import { manager, config } from '../../../../wailsjs/go/models';
import { DeploySSHKey } from '../../../../wailsjs/go/manager/Manager';
import { ListSSHKeys } from '../../../../wailsjs/go/main/App';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string, targetName?: string) => void;
}

export default function DeploySSHKeyTab({ vm, onJobStarted }: Props) {
    const [keys,      setKeys]      = useState<config.KeyMeta[]>([]);
    const [label,     setLabel]     = useState('');
    const [host,      setHost]      = useState(vm.ipAddress || '');
    const [port,      setPort]      = useState(22);
    const [username,  setUsername]  = useState('');
    const [password,  setPassword]  = useState('');
    const [busy,      setBusy]      = useState(false);
    const [error,     setError]     = useState('');
    const [keysError, setKeysError] = useState('');

    useEffect(() => {
        setHost(vm.ipAddress || '');
    }, [vm.ref, vm.ipAddress]);

    useEffect(() => {
        ListSSHKeys()
            .then(list => {
                const l = list ?? [];
                setKeys(l);
                if (l.length > 0) {
                    setLabel(l[0].label);
                    setUsername(l[0].defaultUser || '');
                }
            })
            .catch((e: any) => setKeysError(String(e)));
    }, []);

    // When selected key changes, pre-fill username from its defaultUser.
    useEffect(() => {
        const k = keys.find(k => k.label === label);
        if (k?.defaultUser) setUsername(k.defaultUser);
    }, [label]);

    async function handleDeploy() {
        setError('');
        setBusy(true);
        try {
            const id = await DeploySSHKey({
                label,
                host,
                port,
                username,
                password,
                guestOS: vm.guestOS,
            });
            onJobStarted(id, vm.name || vm.ref);
        } catch (e: any) {
            setError(String(e));
        } finally {
            setBusy(false);
        }
    }

    const selectedKey = keys.find(k => k.label === label);
    const canDeploy = !busy && !!label && !!host && port > 0 && !!username;

    if (keysError) {
        return <div className="tab-body"><p className="form-error">{keysError}</p></div>;
    }

    return (
        <div className="tab-body tab-body--fill tab-body--centered">
            <div className="form-section tab-stack">

                {keys.length === 0 ? (
                    <div className="notice notice--warn">
                        No SSH keys found. Create one in the SSH Keys panel first.
                    </div>
                ) : (
                    <>
                        <div className="field">
                            <label>Key</label>
                            <select value={label} onChange={e => setLabel(e.target.value)}>
                                {keys.map(k => (
                                    <option key={k.label} value={k.label}>
                                        {k.label} ({k.algorithm})
                                    </option>
                                ))}
                            </select>
                        </div>

                        {selectedKey && (
                            <div className="ssh-deploy-pubkey-preview">
                                <span className="ssh-key-badge">{selectedKey.algorithm}</span>
                                <code className="ssh-deploy-pubkey-snippet" title={selectedKey.publicKey}>
                                    {selectedKey.publicKey.slice(0, 48)}…
                                </code>
                            </div>
                        )}

                        <div className="cred-row" style={{ marginTop: '0.75rem' }}>
                            <div className="field field--inline">
                                <label>Host</label>
                                <input
                                    value={host}
                                    onChange={e => setHost(e.target.value)}
                                    placeholder="192.168.1.100"
                                    autoComplete="off"
                                />
                            </div>
                            <div className="field field--inline field--narrow">
                                <label>Port</label>
                                <input
                                    type="number"
                                    value={port}
                                    onChange={e => setPort(parseInt(e.target.value) || 22)}
                                    min={1} max={65535}
                                />
                            </div>
                        </div>

                        <div className="cred-row">
                            <div className="field field--inline">
                                <label>Username</label>
                                <input
                                    value={username}
                                    onChange={e => setUsername(e.target.value)}
                                    placeholder="root"
                                    autoComplete="off"
                                />
                                <span className="field-help">
                                    Used for future SSH/SFTP with this key.
                                </span>
                            </div>
                            <div className="field field--inline">
                                <label>Password</label>
                                <input
                                    type="password"
                                    value={password}
                                    onChange={e => setPassword(e.target.value)}
                                    autoComplete="off"
                                />
                            </div>
                        </div>

                        {error && <p className="form-error">{error}</p>}

                        <button className="btn-primary" onClick={handleDeploy} disabled={!canDeploy}>
                            {busy ? 'Deploying…' : 'Deploy Key'}
                        </button>
                    </>
                )}
            </div>
        </div>
    );
}
