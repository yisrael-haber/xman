import { useState, useEffect } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { DeploySSHKey } from '../../../../wailsjs/go/manager/API';
import type { VMTransportState } from '../../../hooks/useVMTransport';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string, targetName?: string) => void;
    transport: VMTransportState;
}

export default function DeploySSHKeyTab({ vm, onJobStarted, transport }: Props) {
    const [port,      setPort]      = useState(22);
    const [username,  setUsername]  = useState('');
    const [password,  setPassword]  = useState('');
    const [busy,      setBusy]      = useState(false);
    const [error,     setError]     = useState('');

    useEffect(() => {
        if (!transport.keys.length) {
            setUsername('');
            return;
        }
        setUsername(transport.selectedKey?.defaultUser || '');
    }, [transport.keyLabel, transport.keys.length, transport.selectedKey?.defaultUser]);

    async function handleDeploy() {
        setError('');
        setBusy(true);
        try {
            const id = await DeploySSHKey({
                label: transport.keyLabel,
                host: transport.sshHost,
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

    const canDeploy = !busy && !!transport.keyLabel && !!transport.sshHost && port > 0 && !!username;

    if (transport.keysError) {
        return <div className="tab-body"><p className="form-error">{transport.keysError}</p></div>;
    }

    return (
        <div className="tab-body tab-body--fill tab-body--centered">
            <div className="form-section tab-stack">

                {transport.keys.length === 0 ? (
                    <div className="notice notice--warn">
                        No SSH keys found. Create one in the SSH Keys panel first.
                    </div>
                ) : (
                    <>
                        <div className="field">
                            <label>Key</label>
                            <select value={transport.keyLabel} onChange={e => transport.setKeyLabel(e.target.value)}>
                                {transport.keys.map(k => (
                                    <option key={k.label} value={k.label}>
                                        {k.label} ({k.algorithm})
                                    </option>
                                ))}
                            </select>
                        </div>

                        {transport.selectedKey && (
                            <div className="ssh-deploy-pubkey-preview">
                                <span className="ssh-key-badge">{transport.selectedKey.algorithm}</span>
                                <code className="ssh-deploy-pubkey-snippet" title={transport.selectedKey.publicKey}>
                                    {transport.selectedKey.publicKey.slice(0, 48)}…
                                </code>
                            </div>
                        )}

                        <div className="cred-row" style={{ marginTop: '0.75rem' }}>
                            <div className="field field--inline">
                                <label>Host</label>
                                <input
                                    value={transport.sshHost}
                                    onChange={e => transport.setSshHost(e.target.value)}
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
