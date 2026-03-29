import { useState, useRef, useEffect } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { GuestRun, SSHRun } from '../../../../wailsjs/go/manager/Manager';
import { JobCancel } from '../../../../wailsjs/go/jobs/Manager';
import { LaunchInteractiveSSHSession } from '../../../../wailsjs/go/main/App';
import { ClipboardSetText, EventsOn } from '../../../../wailsjs/runtime/runtime';
import useSSHKeys from '../../../hooks/useSSHKeys';
import useGuestCredentials from '../../../hooks/useGuestCredentials';

type Mode = 'vmware' | 'ssh';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string, targetName?: string) => void;
}

interface OutputEntry {
    command: string;
    output: string;
    status: 'running' | 'done' | 'failed' | 'cancelled';
}

function extractCommandOutput(job: any): string {
    const log = Array.isArray(job?.log) ? job.log : [];
    for (let i = log.length - 1; i >= 0; i -= 1) {
        const entry = log[i];
        if (entry?.progress === 95 && typeof entry.message === 'string' && entry.message.trim()) {
            return entry.message;
        }
    }

    if (job?.status === 'cancelled') {
        return 'Command cancelled.';
    }

    if (typeof job?.error === 'string' && job.error.trim()) {
        return job.error;
    }

    if (typeof job?.message === 'string' && job.message.trim()) {
        return job.message;
    }

    return '(no output)';
}

export default function GuestExecTab({ vm, onJobStarted }: Props) {
    const [mode,      setMode]      = useState<Mode>('vmware');
    const [credentialLabel, setCredentialLabel] = useState('');
    const [sshHost,   setSshHost]   = useState(vm.ipAddress || '');
    const [keyLabel,  setKeyLabel]  = useState('');
    const [command,   setCommand]   = useState('');
    const [result,    setResult]    = useState<OutputEntry | null>(null);
    const [busy,      setBusy]      = useState(false);
    const [error,     setError]     = useState('');
    const [copied,    setCopied]    = useState(false);
    const [activeJobId, setActiveJobId] = useState('');
    const [launchingSession, setLaunchingSession] = useState(false);
    const outputRef = useRef<HTMLDivElement>(null);
    const { keys, error: keysError } = useSSHKeys();
    const { credentials, error: credentialsError } = useGuestCredentials();

    // Keep SSH host in sync with the selected VM's IP (including when it appears after boot).
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

    useEffect(() => {
        if (outputRef.current) {
            outputRef.current.scrollTop = 0;
        }
        setCopied(false);
    }, [result]);

    useEffect(() => {
        setResult(null);
        setCommand('');
        setError('');
        setCopied(false);
        setActiveJobId('');
        setLaunchingSession(false);
    }, [vm.ref]);

    const selectedKey = keys.find(k => k.label === keyLabel);
    const selectedCredential = credentials.find(c => c.label === credentialLabel);
    const sshUser = selectedKey?.defaultUser?.trim() || '';
    const guestOpsReady = vm.powerState === 'poweredOn';
    const toolsReady = vm.toolsStatus === 'toolsOk' || vm.toolsStatus === 'toolsOld';

    async function handleRun() {
        if (!command.trim()) return;
        setError('');
        setBusy(true);
        const cmd = command.trim();
        setResult({ command: cmd, output: '', status: 'running' });

        try {
            let id: string;
            if (mode === 'ssh') {
                id = await SSHRun({ host: sshHost, keyLabel, command: cmd });
            } else {
                id = await GuestRun({ vmRef: vm.ref, credentialLabel, username: '', password: '', command: cmd, guestOS: vm.guestOS });
            }
            setActiveJobId(id);
            onJobStarted(id, vm.name || vm.ref);

            const unsub = EventsOn(`job:${id}`, (job: any) => {
                if (job.status === 'done') {
                    setResult({ command: cmd, output: extractCommandOutput(job), status: 'done' });
                    setActiveJobId('');
                    unsub();
                    setBusy(false);
                } else if (job.status === 'failed') {
                    setResult({ command: cmd, output: extractCommandOutput(job), status: 'failed' });
                    setActiveJobId('');
                    unsub();
                    setBusy(false);
                } else if (job.status === 'cancelled') {
                    setResult({ command: cmd, output: extractCommandOutput(job), status: 'cancelled' });
                    setActiveJobId('');
                    unsub();
                    setBusy(false);
                }
            });

            setCommand('');
        } catch (e: any) {
            setError(String(e));
            setResult(null);
            setActiveJobId('');
            setBusy(false);
        }
    }

    async function handleCopy() {
        if (!result || !result.output) return;
        try {
            await ClipboardSetText(result.output);
            setCopied(true);
            window.setTimeout(() => setCopied(false), 2000);
        } catch {
            try {
                await navigator.clipboard.writeText(result.output);
                setCopied(true);
                window.setTimeout(() => setCopied(false), 2000);
            } catch (e: any) {
                setError(String(e));
            }
        }
    }

    function handleCancel() {
        if (!activeJobId) return;
        setError('');
        void JobCancel(activeJobId);
    }

    async function handleLaunchSession() {
        if (!sshReady || mode !== 'ssh') return;
        setError('');
        setLaunchingSession(true);
        try {
            await LaunchInteractiveSSHSession(sshHost, keyLabel);
        } catch (e: any) {
            setError(String(e));
        } finally {
            setLaunchingSession(false);
        }
    }

    function handleKeyDown(e: React.KeyboardEvent) {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            if (!busy) handleRun();
        }
    }

    const sshReady = !!sshHost.trim() && !!keyLabel && !!sshUser;
    const canRun   = !busy && !!command.trim() &&
                     (mode === 'vmware' ? (!!credentialLabel && guestOpsReady) : sshReady);

    return (
        <div className="tab-body tab-body--fill tab-body--centered">
            <div className="form-section exec-credentials">
                <div className="mode-toggle" style={{ alignSelf: 'center' }}>
                    <button className={`mode-btn ${mode === 'vmware' ? 'mode-btn--active' : ''}`}
                        onClick={() => setMode('vmware')}>Guest Ops</button>
                    <button className={`mode-btn ${mode === 'ssh' ? 'mode-btn--active' : ''}`}
                        onClick={() => setMode('ssh')}>SSH</button>
                </div>

                {mode === 'vmware' && !guestOpsReady && (
                    <div className="notice notice--warn">
                        Guest operations require the VM to be powered on.
                    </div>
                )}

                {mode === 'vmware' && guestOpsReady && !toolsReady && (
                    <div className="notice notice--warn">
                        This VM is powered on, so guest operations can be attempted even though VMware Tools may not be ready yet. If the backend rejects the command, the error will appear here.
                    </div>
                )}

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

                        <div className="exec-live-session-panel">
                            <div className="exec-live-session-copy">
                                <span className="exec-live-session-title">Interactive SSH Session</span>
                                <span className="exec-live-session-text">
                                    Open your native terminal with a real SSH shell. This is separate from one-off command execution below.
                                </span>
                            </div>
                            <button
                                className="btn-secondary"
                                onClick={() => void handleLaunchSession()}
                                disabled={!sshReady || launchingSession}
                            >
                                {launchingSession ? 'Launching…' : 'Launch Interactive Session'}
                            </button>
                        </div>
                    </>
                )}
            </div>

            <div className="exec-workspace">
                <div className="exec-session-note">
                    Each command runs in a fresh shell session. Output below replaces the previous run.
                </div>

                <div className="exec-shell-header">
                    <div className="exec-shell-meta">
                        <span className="exec-shell-title">Command Output</span>
                        {result?.command && <span className="exec-shell-command">$ {result.command}</span>}
                    </div>
                    <button
                        className="btn-secondary"
                        onClick={() => void handleCopy()}
                        disabled={!result?.output || result?.status === 'running'}
                    >
                        {copied ? 'Copied!' : 'Copy Output'}
                    </button>
                </div>

                <div className="exec-output" ref={outputRef}>
                    {!result && (
                        <span className="exec-output-empty">Run a command to see output here.</span>
                    )}
                    {result?.status === 'running' && (
                        <div className="exec-entry">
                            <div className="exec-output-text exec-output-running">Running…</div>
                        </div>
                    )}
                    {result && result.status !== 'running' && (
                        <div className="exec-entry">
                            <pre className={`exec-output-text ${result.status === 'failed' ? 'exec-output-error' : result.status === 'cancelled' ? 'exec-output-cancelled' : ''}`}>
                                {result.output || '(no output)'}
                            </pre>
                        </div>
                    )}
                </div>

                <div className="exec-input-row">
                    <span className="exec-prompt-sym">$</span>
                    <input
                        className="exec-input"
                        value={command}
                        onChange={e => setCommand(e.target.value)}
                        onKeyDown={handleKeyDown}
                        placeholder="command to run in guest"
                        disabled={busy || (mode === 'vmware' ? (!credentialLabel || !guestOpsReady) : !sshReady)}
                        autoComplete="off"
                    />
                    <button className="btn-primary" onClick={handleRun} disabled={!canRun}>
                        {busy ? 'Running…' : 'Run'}
                    </button>
                    {busy && activeJobId && (
                        <button className="btn-secondary" onClick={handleCancel}>
                            Cancel
                        </button>
                    )}
                </div>
            </div>

            {error && <p className="form-error">{error}</p>}
        </div>
    );
}
