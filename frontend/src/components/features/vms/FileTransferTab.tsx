import { useState, useEffect } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { Upload, Download, SSHUpload, SSHDownload } from '../../../../wailsjs/go/manager/Manager';
import { OpenFileDialog, SaveFileDialog } from '../../../../wailsjs/go/main/App';
import useSSHKeys from '../../../hooks/useSSHKeys';
import useGuestCredentials from '../../../hooks/useGuestCredentials';

type Mode = 'vmware' | 'ssh';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string, targetName?: string) => void;
}


export default function FileTransferTab({ vm, onJobStarted }: Props) {
    const [mode, setMode] = useState<Mode>('vmware');
    const [credentialLabel, setCredentialLabel] = useState('');

    // SSH key-based params
    const [sshHost, setSshHost] = useState(vm.ipAddress || '');
    const [keyLabel, setKeyLabel] = useState('');
    const { keys, error: keysError } = useSSHKeys();
    const { credentials, error: credentialsError } = useGuestCredentials();

    // Keep SSH host in sync with selected VM's IP (including when it appears after boot).
    useEffect(() => { setSshHost(vm.ipAddress || ''); }, [vm.ref, vm.ipAddress]);

    useEffect(() => {
        if (!keys.length) {
            setKeyLabel('');
            return;
        }
        if (!keyLabel || !keys.some(k => k.label === keyLabel)) {
            setKeyLabel(keys[0].label);
        }
    }, [keys, keyLabel]);

    useEffect(() => {
        if (!credentials.length) {
            setCredentialLabel('');
            return;
        }
        if (!credentialLabel || !credentials.some(c => c.label === credentialLabel)) {
            setCredentialLabel(credentials[0].label);
        }
    }, [credentials, credentialLabel]);

    const [upLocal, setUpLocal] = useState('');
    const [upGuest, setUpGuest] = useState('');
    const [upBusy,  setUpBusy]  = useState(false);
    const [upErr,   setUpErr]   = useState('');

    const [dlGuest, setDlGuest] = useState('');
    const [dlLocal, setDlLocal] = useState('');
    const [dlBusy,  setDlBusy]  = useState(false);
    const [dlErr,   setDlErr]   = useState('');

    const selectedKey = keys.find(k => k.label === keyLabel);
    const selectedCredential = credentials.find(c => c.label === credentialLabel);
    const sshUser = selectedKey?.defaultUser?.trim() || '';
    const guestOpsReady = vm.powerState === 'poweredOn';
    const toolsReady = vm.toolsStatus === 'toolsOk' || vm.toolsStatus === 'toolsOld';

    async function pickUploadFile() {
        const path = await OpenFileDialog('Select file to upload');
        if (path) setUpLocal(path);
    }

    async function pickDownloadDest() {
        const name = dlGuest.split('/').pop() || dlGuest.split('\\').pop() || 'download';
        const path = await SaveFileDialog('Save downloaded file', name);
        if (path) setDlLocal(path);
    }

    async function handleUpload() {
        setUpErr(''); setUpBusy(true);
        try {
            let id: string;
            if (mode === 'ssh') {
                id = await SSHUpload({ host: sshHost, keyLabel, localPath: upLocal, guestPath: upGuest });
            } else {
                id = await Upload({ vmRef: vm.ref, credentialLabel, username: '', password: '', localPath: upLocal, guestPath: upGuest, guestOS: vm.guestOS });
            }
            onJobStarted(id, vm.name || vm.ref);
        } catch (e: any) {
            setUpErr(String(e));
        } finally {
            setUpBusy(false);
        }
    }

    async function handleDownload() {
        setDlErr(''); setDlBusy(true);
        try {
            let id: string;
            if (mode === 'ssh') {
                id = await SSHDownload({ host: sshHost, keyLabel, guestPath: dlGuest, localPath: dlLocal });
            } else {
                id = await Download({ vmRef: vm.ref, credentialLabel, username: '', password: '', guestPath: dlGuest, localPath: dlLocal });
            }
            onJobStarted(id, vm.name || vm.ref);
        } catch (e: any) {
            setDlErr(String(e));
        } finally {
            setDlBusy(false);
        }
    }

    const sshReady = !!sshHost.trim() && !!keyLabel && !!sshUser;

    return (
        <div className="tab-body tab-body--fill tab-body--centered">
            <div className="form-section tab-stack" style={{ flexShrink: 0 }}>
                <div className="mode-toggle" style={{ alignSelf: 'center' }}>
                    <button className={`mode-btn ${mode === 'vmware' ? 'mode-btn--active' : ''}`}
                        onClick={() => setMode('vmware')}>Guest Ops</button>
                    <button className={`mode-btn ${mode === 'ssh' ? 'mode-btn--active' : ''}`}
                        onClick={() => setMode('ssh')}>SSH / SFTP</button>
                </div>

                {mode === 'vmware' ? (
                    <>
                        {credentialsError && <p className="form-error">{credentialsError}</p>}
                        {!credentialsError && credentials.length === 0 && (
                            <div className="notice notice--warn">
                                No guest credentials found. Create one in Guest Credentials first.
                            </div>
                        )}

                        <div className="cred-row">
                            <div className="field field--inline">
                                <label>Credential</label>
                                <select value={credentialLabel} onChange={e => setCredentialLabel(e.target.value)} disabled={!credentials.length}>
                                    {credentials.map(credential => (
                                        <option key={credential.label} value={credential.label}>
                                            {credential.label} ({credential.username})
                                        </option>
                                    ))}
                                </select>
                            </div>
                            <div className="field field--inline">
                                <label>Username</label>
                                <input value={selectedCredential?.username ?? ''} readOnly placeholder="Select a stored credential" />
                            </div>
                        </div>
                    </>
                ) : (
                    <>
                        {keysError && <p className="form-error">{keysError}</p>}
                        {!keysError && keys.length === 0 && (
                            <div className="notice notice--warn">
                                No SSH keys found. Create one in SSH Keys first.
                            </div>
                        )}

                        <div className="cred-row">
                            <div className="field field--inline">
                                <label>Host</label>
                                <input value={sshHost} onChange={e => setSshHost(e.target.value)}
                                    placeholder="192.168.1.100" autoComplete="off" />
                            </div>
                            <div className="field field--inline">
                                <label>Key</label>
                                <select value={keyLabel} onChange={e => setKeyLabel(e.target.value)} disabled={!keys.length}>
                                    {keys.map(k => (
                                        <option key={k.label} value={k.label}>
                                            {k.label} ({k.algorithm})
                                        </option>
                                    ))}
                                </select>
                            </div>
                        </div>

                        <div className="cred-row">
                            <div className="field field--inline">
                                <label>User</label>
                                <input value={sshUser} readOnly placeholder="Set a default user on the selected key" />
                            </div>
                        </div>

                        {selectedKey && !sshUser && (
                            <div className="notice notice--warn">
                                The selected key has no default user. Set one in SSH Keys or choose a different key.
                            </div>
                        )}
                    </>
                )}

                {mode === 'vmware' && !guestOpsReady && (
                    <div className="notice notice--warn">
                        Guest operations require the VM to be powered on.
                    </div>
                )}

                {mode === 'vmware' && guestOpsReady && !toolsReady && (
                    <div className="notice notice--warn">
                        This VM is powered on, so guest operations can be attempted even though VMware Tools may not be ready yet. If the backend rejects the transfer, the error will appear here.
                    </div>
                )}
            </div>

            <div className="tab-split-panels">
                {/* Upload */}
                <div className="transfer-panel">
                    <h3 className="transfer-title">Upload to guest</h3>
                    <div className="field">
                        <label>Local file</label>
                        <div className="input-with-btn">
                            <input value={upLocal} readOnly placeholder="Select a file..." />
                            <button className="btn-secondary" onClick={pickUploadFile}>Browse</button>
                        </div>
                    </div>
                    <div className="field">
                        <label>Guest destination path</label>
                        <input value={upGuest} onChange={e => setUpGuest(e.target.value)}
                            placeholder="/tmp/file.txt" />
                    </div>
                    {upErr && <p className="form-error">{upErr}</p>}
                    <button className="btn-primary" onClick={handleUpload}
                        disabled={upBusy || !upLocal || !upGuest ||
                            (mode === 'vmware' ? (!credentialLabel || !guestOpsReady) : !sshReady)}>
                        {upBusy ? 'Starting...' : 'Upload'}
                    </button>
                </div>

                <div className="transfer-divider" />

                {/* Download */}
                <div className="transfer-panel">
                    <h3 className="transfer-title">Download from guest</h3>
                    <div className="field">
                        <label>Guest source path</label>
                        <input value={dlGuest} onChange={e => setDlGuest(e.target.value)}
                            placeholder="/var/log/syslog" />
                    </div>
                    <div className="field">
                        <label>Save to</label>
                        <div className="input-with-btn">
                            <input value={dlLocal} readOnly placeholder="Choose save location..." />
                            <button className="btn-secondary" onClick={pickDownloadDest}>Browse</button>
                        </div>
                    </div>
                    {dlErr && <p className="form-error">{dlErr}</p>}
                    <button className="btn-primary" onClick={handleDownload}
                        disabled={dlBusy || !dlGuest || !dlLocal ||
                            (mode === 'vmware' ? (!credentialLabel || !guestOpsReady) : !sshReady)}>
                        {dlBusy ? 'Starting...' : 'Download'}
                    </button>
                </div>
            </div>
        </div>
    );
}
