import { useState } from 'react';
import { vminfo } from '../../../../wailsjs/go/models';
import { StartCapture } from '../../../../wailsjs/go/packetcapture/Binding';
import { SaveFileDialog } from '../../../../wailsjs/go/main/App';

interface Props {
    vm: vminfo.VMInfo;
    onJobStarted: (id: string) => void;
}

export default function PacketCaptureTab({ vm, onJobStarted }: Props) {
    const [username,  setUsername]  = useState('');
    const [password,  setPassword]  = useState('');
    const [iface,     setIface]     = useState('');
    const [duration,  setDuration]  = useState(30);
    const [localPath, setLocalPath] = useState('');
    const [busy,      setBusy]      = useState(false);
    const [error,     setError]     = useState('');

    async function pickSavePath() {
        const path = await SaveFileDialog('Save packet capture', `capture_${vm.name}.pcap`);
        if (path) setLocalPath(path);
    }

    async function handleCapture() {
        setError(''); setBusy(true);
        try {
            const id = await StartCapture({
                vmRef:     vm.ref,
                username,
                password,
                interface: iface,
                duration,
                localPath,
            });
            onJobStarted(id);
        } catch (e: any) {
            setError(String(e));
        } finally {
            setBusy(false);
        }
    }

    const toolsOk = vm.toolsStatus === 'toolsOk' || vm.toolsStatus === 'toolsOld';

    if (!toolsOk) {
        return (
            <div className="tab-body">
                <div className="notice notice--warn">
                    VMware Tools are not running on this VM. Packet capture requires Tools to be installed and running.
                </div>
            </div>
        );
    }

    return (
        <div className="tab-body">
            <div className="form-section">
                <div className="cred-row">
                    <div className="field field--inline">
                        <label>Guest user</label>
                        <input value={username} onChange={e => setUsername(e.target.value)}
                            placeholder="root" autoComplete="off" />
                    </div>
                    <div className="field field--inline">
                        <label>Guest password</label>
                        <input type="password" value={password} onChange={e => setPassword(e.target.value)}
                            autoComplete="off" />
                    </div>
                </div>

                <div className="cred-row">
                    <div className="field field--inline">
                        <label>Interface</label>
                        <input value={iface} onChange={e => setIface(e.target.value)}
                            placeholder="any" />
                    </div>
                    <div className="field field--inline">
                        <label>Duration (seconds)</label>
                        <input type="number" min={1} max={300} value={duration}
                            onChange={e => setDuration(Number(e.target.value))} />
                    </div>
                </div>

                <div className="field">
                    <label>Save .pcap to</label>
                    <div className="input-with-btn">
                        <input value={localPath} readOnly placeholder="Choose save location..." />
                        <button className="btn-secondary" onClick={pickSavePath}>Browse</button>
                    </div>
                </div>

                {error && <p className="form-error">{error}</p>}

                <button className="btn-primary" onClick={handleCapture}
                    disabled={busy || !username || !localPath}>
                    {busy ? 'Starting...' : `Capture ${duration}s`}
                </button>
            </div>
        </div>
    );
}
