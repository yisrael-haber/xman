import type { VMTransportMode, VMTransportState } from '../../../hooks/useVMTransport';

interface Props {
    transport: VMTransportState;
}

export default function VMTransportControls({ transport }: Props) {
    const {
        mode,
        setMode,
        credentialLabel,
        setCredentialLabel,
        sshHost,
        setSshHost,
        keyLabel,
        setKeyLabel,
        keys,
        keysError,
        credentials,
        credentialsError,
        selectedKey,
        sshUser,
        toolsStatus,
        vmPoweredOn,
        guestOpsReady,
    } = transport;

    let message = '';
    let messageTone: 'info' | 'warn' | 'error' = 'info';

    if (mode === 'vmware') {
        if (credentialsError) {
            message = credentialsError;
            messageTone = 'error';
        } else if (credentials.length === 0) {
            message = 'No guest credentials available.';
            messageTone = 'warn';
        } else if (!vmPoweredOn) {
            message = 'Requires a powered-on VM.';
            messageTone = 'warn';
        } else if (toolsStatus === 'toolsNotInstalled') {
            message = 'Requires VMware Tools.';
            messageTone = 'warn';
        } else if (!guestOpsReady) {
            message = 'Guest Ops is not ready yet.';
            messageTone = 'warn';
        }
    } else {
        if (keysError) {
            message = keysError;
            messageTone = 'error';
        } else if (keys.length === 0) {
            message = 'No SSH keys available.';
            messageTone = 'warn';
        } else if (selectedKey && !sshUser) {
            message = 'Selected key needs a default user.';
            messageTone = 'warn';
        }
    }

    return (
        <div className="vm-transport-panel">
            <div className="vm-transport-row">
                <div className="field vm-transport-field vm-transport-field--mode">
                    <label>Transport</label>
                    <select value={mode} onChange={e => setMode(e.target.value as VMTransportMode)}>
                        <option value="vmware">Guest Ops</option>
                        <option value="ssh">SSH / SFTP</option>
                    </select>
                </div>

                {mode === 'vmware' ? (
                    <>
                        {credentials.length > 0 && (
                            <div className="field vm-transport-field vm-transport-field--wide">
                                <label>Credential</label>
                                <select
                                    value={credentialLabel}
                                    onChange={e => setCredentialLabel(e.target.value)}
                                    disabled={!credentials.length}
                                >
                                    {credentials.map(credential => (
                                        <option key={credential.label} value={credential.label}>
                                            {credential.label} ({credential.username})
                                        </option>
                                    ))}
                                </select>
                            </div>
                        )}
                    </>
                ) : (
                    <>
                        {keys.length > 0 && (
                            <>
                                <div className="field vm-transport-field vm-transport-field--wide">
                                    <label>Host</label>
                                    <input
                                        value={sshHost}
                                        onChange={e => setSshHost(e.target.value)}
                                        placeholder="192.168.1.100"
                                        autoComplete="off"
                                    />
                                </div>
                                <div className="field vm-transport-field">
                                    <label>Key</label>
                                    <select value={keyLabel} onChange={e => setKeyLabel(e.target.value)} disabled={!keys.length}>
                                        {keys.map(key => (
                                            <option key={key.label} value={key.label}>
                                                {key.label}{key.defaultUser ? ` (${key.defaultUser})` : ' (no user)'}
                                            </option>
                                        ))}
                                    </select>
                                </div>
                            </>
                        )}
                    </>
                )}
            </div>

            {message && (
                <p className={`vm-transport-message vm-transport-message--${messageTone}`}>
                    {message}
                </p>
            )}
        </div>
    );
}
