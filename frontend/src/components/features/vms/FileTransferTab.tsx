import { useState } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { Upload, Download, SSHUpload, SSHDownload } from '../../../../wailsjs/go/manager/Manager';
import { OpenFileDialog, SaveFileDialog } from '../../../../wailsjs/go/main/App';
import type { VMTransportState } from '../../../hooks/useVMTransport';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string, targetName?: string) => void;
    transport: VMTransportState;
}


export default function FileTransferTab({ vm, onJobStarted, transport }: Props) {
    const { mode, credentialLabel, sshHost, keyLabel, sshUser, vmPoweredOn } = transport;

    const [upLocal, setUpLocal] = useState('');
    const [upGuest, setUpGuest] = useState('');
    const [upBusy,  setUpBusy]  = useState(false);
    const [upErr,   setUpErr]   = useState('');

    const [dlGuest, setDlGuest] = useState('');
    const [dlLocal, setDlLocal] = useState('');
    const [dlBusy,  setDlBusy]  = useState(false);
    const [dlErr,   setDlErr]   = useState('');

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
            <div className="tab-split-panels">
                <div className="transfer-panel">
                    <h3 className="transfer-title">Upload</h3>
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
                            (mode === 'vmware' ? (!credentialLabel || !vmPoweredOn) : !sshReady)}>
                        {upBusy ? 'Starting...' : 'Upload'}
                    </button>
                </div>

                <div className="transfer-panel">
                    <h3 className="transfer-title">Download</h3>
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
                            (mode === 'vmware' ? (!credentialLabel || !vmPoweredOn) : !sshReady)}>
                        {dlBusy ? 'Starting...' : 'Download'}
                    </button>
                </div>
            </div>
        </div>
    );
}
