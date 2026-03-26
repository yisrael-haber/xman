import { useState } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { Upload, Download } from '../../../../wailsjs/go/manager/Manager';
import { OpenFileDialog, SaveFileDialog } from '../../../../wailsjs/go/main/App';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string) => void;
}

function GuestCredentials({ username, password, onChange }: {
    username: string; password: string;
    onChange: (u: string, p: string) => void;
}) {
    return (
        <div className="cred-row">
            <div className="field field--inline">
                <label>Guest user</label>
                <input value={username} onChange={e => onChange(e.target.value, password)}
                    placeholder="root" autoComplete="off" />
            </div>
            <div className="field field--inline">
                <label>Guest password</label>
                <input type="password" value={password} onChange={e => onChange(username, e.target.value)}
                    autoComplete="off" />
            </div>
        </div>
    );
}

export default function FileTransferTab({ vm, onJobStarted }: Props) {
    const [upUser, setUpUser]   = useState('');
    const [upPass, setUpPass]   = useState('');
    const [upLocal, setUpLocal] = useState('');
    const [upGuest, setUpGuest] = useState('');
    const [upBusy, setUpBusy]   = useState(false);
    const [upErr, setUpErr]     = useState('');

    const [dlUser, setDlUser]   = useState('');
    const [dlPass, setDlPass]   = useState('');
    const [dlGuest, setDlGuest] = useState('');
    const [dlLocal, setDlLocal] = useState('');
    const [dlBusy, setDlBusy]   = useState(false);
    const [dlErr, setDlErr]     = useState('');

    async function pickUploadFile() {
        const path = await OpenFileDialog('Select file to upload');
        if (path) setUpLocal(path);
    }

    async function pickDownloadDest() {
        const name = dlGuest.split('/').pop() || 'download';
        const path = await SaveFileDialog('Save downloaded file', name);
        if (path) setDlLocal(path);
    }

    async function handleUpload() {
        setUpErr(''); setUpBusy(true);
        try {
            const id = await Upload({ vmRef: vm.ref, username: upUser, password: upPass, localPath: upLocal, guestPath: upGuest });
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
            const id = await Download({ vmRef: vm.ref, username: dlUser, password: dlPass, guestPath: dlGuest, localPath: dlLocal });
            onJobStarted(id);
        } catch (e: any) {
            setDlErr(String(e));
        } finally {
            setDlBusy(false);
        }
    }

    const toolsOk = vm.toolsStatus === 'toolsOk' || vm.toolsStatus === 'toolsOld';

    if (!toolsOk) {
        return (
            <div className="tab-body">
                <div className="notice notice--warn">
                    VMware Tools are not running on this VM. File transfer requires Tools to be installed and running.
                </div>
            </div>
        );
    }

    return (
        <div className="tab-body tab-body--split">
            {/* Upload */}
            <div className="transfer-panel">
                <h3 className="transfer-title">Upload to guest</h3>
                <GuestCredentials username={upUser} password={upPass} onChange={(u, p) => { setUpUser(u); setUpPass(p); }} />
                <div className="field">
                    <label>Local file</label>
                    <div className="input-with-btn">
                        <input value={upLocal} readOnly placeholder="Select a file..." />
                        <button className="btn-secondary" onClick={pickUploadFile}>Browse</button>
                    </div>
                </div>
                <div className="field">
                    <label>Guest destination path</label>
                    <input value={upGuest} onChange={e => setUpGuest(e.target.value)} placeholder="/tmp/file.txt" />
                </div>
                {upErr && <p className="form-error">{upErr}</p>}
                <button className="btn-primary" onClick={handleUpload}
                    disabled={upBusy || !upLocal || !upGuest || !upUser}>
                    {upBusy ? 'Starting...' : 'Upload'}
                </button>
            </div>

            <div className="transfer-divider" />

            {/* Download */}
            <div className="transfer-panel">
                <h3 className="transfer-title">Download from guest</h3>
                <GuestCredentials username={dlUser} password={dlPass} onChange={(u, p) => { setDlUser(u); setDlPass(p); }} />
                <div className="field">
                    <label>Guest source path</label>
                    <input value={dlGuest} onChange={e => setDlGuest(e.target.value)} placeholder="/var/log/syslog" />
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
                    disabled={dlBusy || !dlGuest || !dlLocal || !dlUser}>
                    {dlBusy ? 'Starting...' : 'Download'}
                </button>
            </div>
        </div>
    );
}
