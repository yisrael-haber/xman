import { useState } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { VMDeployAndRun } from '../../../../wailsjs/go/manager/Manager';
import { OpenFileDialog } from '../../../../wailsjs/go/main/App';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string) => void;
}

export default function DeployTab({ vm, onJobStarted }: Props) {
    const [username,   setUsername]   = useState('');
    const [password,   setPassword]   = useState('');
    const [localPath,  setLocalPath]  = useState('');
    const [runCommand, setRunCommand] = useState('');
    const [busy,       setBusy]       = useState(false);
    const [error,      setError]      = useState('');

    const toolsOk = vm.toolsStatus === 'toolsOk' || vm.toolsStatus === 'toolsOld';

    if (!toolsOk) {
        return (
            <div className="tab-body">
                <div className="notice notice--warn">
                    VMware Tools are not running on this VM. Deploying installers requires Tools to be installed and running.
                </div>
            </div>
        );
    }

    async function pickFile() {
        const path = await OpenFileDialog('Select installer');
        if (path) setLocalPath(path);
    }

    async function handleDeploy() {
        setError('');
        setBusy(true);
        try {
            const id = await VMDeployAndRun({
                vmRef:      vm.ref,
                username,
                password,
                localPath,
                runCommand,
            });
            onJobStarted(id);
        } catch (e: any) {
            setError(String(e));
        } finally {
            setBusy(false);
        }
    }

    const canSubmit = !busy && !!localPath && !!username;

    return (
        <div className="tab-body">
            <div className="cred-row">
                <div className="field field--inline">
                    <label>Guest user</label>
                    <input value={username} onChange={e => setUsername(e.target.value)}
                        placeholder="Administrator" autoComplete="off" />
                </div>
                <div className="field field--inline">
                    <label>Guest password</label>
                    <input type="password" value={password} onChange={e => setPassword(e.target.value)}
                        autoComplete="off" />
                </div>
            </div>

            <div className="field">
                <label>Installer file</label>
                <div className="input-with-btn">
                    <input value={localPath} readOnly placeholder="Select an installer..." />
                    <button className="btn-secondary" onClick={pickFile}>Browse</button>
                </div>
            </div>

            <div className="field">
                <label>Run command <span className="field-hint">(leave blank to auto-detect)</span></label>
                <input value={runCommand} onChange={e => setRunCommand(e.target.value)}
                    placeholder={`e.g. msiexec /i "C:\\Windows\\Temp\\file.msi" /qn /norestart`}
                    autoComplete="off" />
            </div>

            {error && <p className="form-error">{error}</p>}

            <button className="btn-primary" onClick={handleDeploy} disabled={!canSubmit}>
                {busy ? 'Submitting…' : 'Deploy & Run'}
            </button>
        </div>
    );
}
