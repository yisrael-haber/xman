import { useState, useEffect } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { Upload, Download, SSHUpload, SSHDownload } from '../../../../wailsjs/go/manager/Manager';
import { OpenFileDialog, SaveFileDialog } from '../../../../wailsjs/go/main/App';

type Mode = 'vmware' | 'ssh';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string) => void;
}


export default function FileTransferTab({ vm, onJobStarted }: Props) {
    const [mode, setMode] = useState<Mode>('vmware');

    // Shared credentials across both modes
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');

    // SSH connection params
    const [sshHost, setSshHost] = useState(vm.ipAddress || '');
    const [sshPort, setSshPort] = useState(22);

    // Keep SSH host in sync with selected VM's IP.
    useEffect(() => { setSshHost(vm.ipAddress || ''); }, [vm.ref]);

    const [upLocal, setUpLocal] = useState('');
    const [upGuest, setUpGuest] = useState('');
    const [upBusy,  setUpBusy]  = useState(false);
    const [upErr,   setUpErr]   = useState('');

    const [dlGuest, setDlGuest] = useState('');
    const [dlLocal, setDlLocal] = useState('');
    const [dlBusy,  setDlBusy]  = useState(false);
    const [dlErr,   setDlErr]   = useState('');

    const toolsOk = vm.toolsStatus === 'toolsOk' || vm.toolsStatus === 'toolsOld';

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
                id = await SSHUpload({ host: sshHost, port: sshPort, username, password, localPath: upLocal, guestPath: upGuest });
            } else {
                id = await Upload({ vmRef: vm.ref, username, password, localPath: upLocal, guestPath: upGuest });
            }
            onJobStarted(id);
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
                id = await SSHDownload({ host: sshHost, port: sshPort, username, password, guestPath: dlGuest, localPath: dlLocal });
            } else {
                id = await Download({ vmRef: vm.ref, username, password, guestPath: dlGuest, localPath: dlLocal });
            }
            onJobStarted(id);
        } catch (e: any) {
            setDlErr(String(e));
        } finally {
            setDlBusy(false);
        }
    }

    const sshReady = !!sshHost && sshPort > 0 && !!username;

    return (
        <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
            <div className="form-section" style={{ padding: '1rem 1.25rem', flexShrink: 0, alignSelf: 'center', width: '420px' }}>
                <div className="mode-toggle" style={{ alignSelf: 'center' }}>
                    <button className={`mode-btn ${mode === 'vmware' ? 'mode-btn--active' : ''}`}
                        onClick={() => setMode('vmware')}>VMware</button>
                    <button className={`mode-btn ${mode === 'ssh' ? 'mode-btn--active' : ''}`}
                        onClick={() => setMode('ssh')}>SSH / SFTP</button>
                </div>

                <div className="cred-row">
                    <div className="field field--inline">
                        <label>Username</label>
                        <input value={username} onChange={e => setUsername(e.target.value)}
                            placeholder="root" autoComplete="off" />
                    </div>
                    <div className="field field--inline">
                        <label>Password</label>
                        <input type="password" value={password} onChange={e => setPassword(e.target.value)}
                            autoComplete="off" />
                    </div>
                </div>

                {mode === 'ssh' && (
                    <div className="cred-row">
                        <div className="field field--inline">
                            <label>Host</label>
                            <input value={sshHost} onChange={e => setSshHost(e.target.value)}
                                placeholder="192.168.1.100" autoComplete="off" />
                        </div>
                        <div className="field field--inline field--narrow">
                            <label>Port</label>
                            <input type="number" value={sshPort}
                                onChange={e => setSshPort(parseInt(e.target.value) || 22)}
                                min={1} max={65535} />
                        </div>
                    </div>
                )}

                {mode === 'vmware' && !toolsOk && (
                    <div className="notice notice--warn">
                        VMware Tools are not running. Switch to SSH/SFTP or start Tools to transfer files.
                    </div>
                )}
            </div>

            <div style={{ flex: 1, minHeight: 0, display: 'flex' }}>
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
                            (mode === 'vmware' ? (!username || !toolsOk) : !sshReady)}>
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
                            (mode === 'vmware' ? (!username || !toolsOk) : !sshReady)}>
                        {dlBusy ? 'Starting...' : 'Download'}
                    </button>
                </div>
            </div>
        </div>
    );
}
