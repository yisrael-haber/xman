import { useState, useRef, useEffect } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { GuestRun, SSHRun } from '../../../../wailsjs/go/manager/Manager';
import { EventsOn, EventsOff } from '../../../../wailsjs/runtime/runtime';

type Mode = 'vmware' | 'ssh';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string) => void;
}

interface OutputEntry {
    command: string;
    output: string;
    status: 'running' | 'done' | 'failed';
}

export default function GuestExecTab({ vm, onJobStarted }: Props) {
    const [mode,     setMode]     = useState<Mode>('vmware');
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [sshHost,  setSshHost]  = useState(vm.ipAddress || '');
    const [sshPort,  setSshPort]  = useState(22);
    const [command,  setCommand]  = useState('');
    const [history,  setHistory]  = useState<OutputEntry[]>([]);
    const [busy,     setBusy]     = useState(false);
    const [error,    setError]    = useState('');
    const outputRef = useRef<HTMLDivElement>(null);

    // Keep SSH host in sync with the selected VM's IP (including when it appears after boot).
    useEffect(() => { setSshHost(vm.ipAddress || ''); }, [vm.ref, vm.ipAddress]);

    useEffect(() => {
        if (outputRef.current) {
            outputRef.current.scrollTop = outputRef.current.scrollHeight;
        }
    }, [history]);

    const toolsOk = vm.toolsStatus === 'toolsOk' || vm.toolsStatus === 'toolsOld';

    async function handleRun() {
        if (!command.trim()) return;
        setError('');
        setBusy(true);
        const cmd = command.trim();
        const idx = history.length;
        const entry: OutputEntry = { command: cmd, output: '', status: 'running' };
        setHistory(prev => [...prev, entry]);

        try {
            let id: string;
            if (mode === 'ssh') {
                id = await SSHRun({ host: sshHost, port: sshPort, username, password, command: cmd });
            } else {
                id = await GuestRun({ vmRef: vm.ref, username, password, command: cmd, guestOS: vm.guestOS });
            }
            onJobStarted(id);

            const unsub = EventsOn(`job:${id}`, (job: any) => {
                if (job.status === 'done') {
                    setHistory(prev => prev.map((e, i) =>
                        i === idx ? { ...e, output: job.message, status: 'done' } : e
                    ));
                    EventsOff(`job:${id}`);
                    unsub();
                    setBusy(false);
                } else if (job.status === 'failed') {
                    setHistory(prev => prev.map((e, i) =>
                        i === idx ? { ...e, output: job.error || 'Command failed', status: 'failed' } : e
                    ));
                    EventsOff(`job:${id}`);
                    unsub();
                    setBusy(false);
                }
            });

            setCommand('');
        } catch (e: any) {
            setError(String(e));
            setHistory(prev => prev.slice(0, -1));
            setBusy(false);
        }
    }

    function handleKeyDown(e: React.KeyboardEvent) {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            if (!busy) handleRun();
        }
    }

    const notReady = mode === 'vmware' && !toolsOk;
    const canRun   = !busy && !!command.trim() && !!username &&
                     (mode === 'vmware' || (!!sshHost && sshPort > 0));

    return (
        <div className="tab-body" style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
            <div className="form-section" style={{ alignSelf: 'center', width: '420px' }}>
                <div className="mode-toggle" style={{ alignSelf: 'center' }}>
                    <button className={`mode-btn ${mode === 'vmware' ? 'mode-btn--active' : ''}`}
                        onClick={() => setMode('vmware')}>VMware</button>
                    <button className={`mode-btn ${mode === 'ssh' ? 'mode-btn--active' : ''}`}
                        onClick={() => setMode('ssh')}>SSH</button>
                </div>

                {mode === 'vmware' && !toolsOk && (
                    <div className="notice notice--warn">
                        VMware Tools are not running. Switch to SSH or start Tools to run commands.
                    </div>
                )}

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
            </div>

            <div className="exec-output" ref={outputRef}>
                {history.length === 0 && (
                    <span className="exec-output-empty">Run a command to see output here.</span>
                )}
                {history.map((entry, i) => (
                    <div key={i} className="exec-entry">
                        <div className="exec-prompt">$ {entry.command}</div>
                        {entry.status === 'running' && (
                            <div className="exec-output-text exec-output-running">Running…</div>
                        )}
                        {entry.status !== 'running' && (
                            <pre className={`exec-output-text ${entry.status === 'failed' ? 'exec-output-error' : ''}`}>
                                {entry.output}
                            </pre>
                        )}
                    </div>
                ))}
            </div>

            {error && <p className="form-error">{error}</p>}

            <div className="exec-input-row">
                <span className="exec-prompt-sym">$</span>
                <input
                    className="exec-input"
                    value={command}
                    onChange={e => setCommand(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder="command to run in guest"
                    disabled={busy || notReady || !username}
                    autoComplete="off"
                />
                <button className="btn-primary" onClick={handleRun} disabled={!canRun || notReady}>
                    {busy ? 'Running…' : 'Run'}
                </button>
            </div>
        </div>
    );
}
