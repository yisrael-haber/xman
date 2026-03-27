import { useState, useEffect } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { Install, SSHInstall } from '../../../../wailsjs/go/manager/Manager';
import { OpenFileDialog } from '../../../../wailsjs/go/main/App';

type Mode = 'vmware' | 'ssh';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string) => void;
}

const SUPPORTED_EXTS = ['msi', 'msix', 'msixbundle', 'exe', 'deb', 'rpm'];

function basename(p: string): string {
    return p.split(/[\\/]/).pop() ?? p;
}

function deriveCommand(localPath: string): string {
    const ext = localPath.split('.').pop()?.toLowerCase() ?? '';
    const filename = basename(localPath);
    const isWin = ['msi', 'msix', 'msixbundle', 'exe'].includes(ext);
    const guestPath = isWin
        ? `C:\\Windows\\Temp\\${filename}`
        : `/tmp/${filename}`;

    switch (ext) {
        case 'msi':        return `msiexec /i "${guestPath}" /qn /norestart`;
        case 'msix':
        case 'msixbundle': return `powershell -Command "Add-AppxPackage -Path '${guestPath}'"`;
        case 'exe':        return `"${guestPath}" /S /SILENT /VERYSILENT /quiet /norestart`;
        case 'deb':        return `DEBIAN_FRONTEND=noninteractive dpkg -i "${guestPath}" && apt-get install -f -y`;
        case 'rpm':        return `rpm -i "${guestPath}"`;
        default:           return '';
    }
}

export default function RemoteInstallTab({ vm, onJobStarted }: Props) {
    const [mode,      setMode]      = useState<Mode>('vmware');
    const [username,  setUsername]  = useState('');
    const [password,  setPassword]  = useState('');
    const [sshHost,   setSshHost]   = useState(vm.ipAddress || '');
    const [sshPort,   setSshPort]   = useState(22);
    const [localPath, setLocalPath] = useState('');
    const [command,   setCommand]   = useState('');
    const [busy,      setBusy]      = useState(false);
    const [error,     setError]     = useState('');

    useEffect(() => { setSshHost(vm.ipAddress || ''); }, [vm.ref, vm.ipAddress]);

    // Auto-derive install command when the selected file changes.
    useEffect(() => {
        setCommand(localPath ? deriveCommand(localPath) : '');
    }, [localPath]);

    const toolsOk = vm.toolsStatus === 'toolsOk' || vm.toolsStatus === 'toolsOld';

    async function pickFile() {
        const p = await OpenFileDialog('Select installer package');
        if (p) setLocalPath(p);
    }

    async function handleInstall() {
        setError(''); setBusy(true);
        try {
            let id: string;
            if (mode === 'ssh') {
                id = await SSHInstall({
                    host: sshHost, port: sshPort,
                    username, password,
                    localPath, guestOS: vm.guestOS,
                    command,
                });
            } else {
                id = await Install({
                    vmRef: vm.ref,
                    username, password,
                    localPath, guestOS: vm.guestOS,
                    command,
                });
            }
            onJobStarted(id);
        } catch (e: any) {
            setError(String(e));
        } finally {
            setBusy(false);
        }
    }

    const ext = localPath.split('.').pop()?.toLowerCase() ?? '';
    const validExt = SUPPORTED_EXTS.includes(ext);
    const sshReady = !!sshHost && sshPort > 0 && !!username;
    const canInstall = !busy && !!localPath && validExt && !!command &&
        (mode === 'ssh' ? sshReady : (!!username && toolsOk));

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
                            placeholder="Administrator" autoComplete="off" />
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
                        VMware Tools are not running. Switch to SSH/SFTP or start Tools to install packages.
                    </div>
                )}
            </div>

            <div style={{
                flex: 1, minHeight: 0, padding: '0 1.25rem 1rem',
                display: 'flex', flexDirection: 'column', gap: '0.75rem',
                alignSelf: 'center', width: '420px', boxSizing: 'border-box',
            }}>
                <div className="field">
                    <label>Installer package</label>
                    <div className="input-with-btn">
                        <input value={localPath} readOnly placeholder="Select .msi, .exe, .deb, or .rpm..." />
                        <button className="btn-secondary" onClick={pickFile}>Browse</button>
                    </div>
                    {localPath && !validExt && (
                        <p className="form-error">Unsupported file type. Use .msi, .exe, .deb, or .rpm.</p>
                    )}
                </div>

                <div className="field">
                    <label>
                        Install command{' '}
                        <span style={{ fontWeight: 400, opacity: 0.6 }}>(auto-detected — edit to override)</span>
                    </label>
                    <textarea
                        value={command}
                        onChange={e => setCommand(e.target.value)}
                        rows={3}
                        placeholder="Select a file to auto-detect the install command..."
                        style={{ fontFamily: 'monospace', fontSize: '0.8rem', resize: 'vertical' }}
                    />
                </div>

                {error && <p className="form-error">{error}</p>}

                <button className="btn-primary" onClick={handleInstall} disabled={!canInstall}>
                    {busy ? 'Starting...' : 'Install'}
                </button>
            </div>
        </div>
    );
}
