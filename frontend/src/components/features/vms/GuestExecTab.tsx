import { useState, useRef, useEffect } from 'react';
import { vminfo } from '../../../../wailsjs/go/models';
import { GuestRun } from '../../../../wailsjs/go/guestexec/Binding';
import { EventsOn, EventsOff } from '../../../../wailsjs/runtime/runtime';

interface Props {
    vm: vminfo.VMInfo;
    onJobStarted: (id: string) => void;
}

interface OutputEntry {
    command: string;
    output: string;
    status: 'running' | 'done' | 'failed';
}

export default function GuestExecTab({ vm, onJobStarted }: Props) {
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [command,  setCommand]  = useState('');
    const [history,  setHistory]  = useState<OutputEntry[]>([]);
    const [busy,     setBusy]     = useState(false);
    const [error,    setError]    = useState('');
    const outputRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (outputRef.current) {
            outputRef.current.scrollTop = outputRef.current.scrollHeight;
        }
    }, [history]);

    const toolsOk = vm.toolsStatus === 'toolsOk' || vm.toolsStatus === 'toolsOld';

    if (!toolsOk) {
        return (
            <div className="tab-body">
                <div className="notice notice--warn">
                    VMware Tools are not running on this VM. Guest command execution requires Tools to be installed and running.
                </div>
            </div>
        );
    }

    async function handleRun() {
        if (!command.trim()) return;
        setError('');
        setBusy(true);
        const cmd = command.trim();
        const entry: OutputEntry = { command: cmd, output: '', status: 'running' };
        setHistory(prev => [...prev, entry]);
        const idx = history.length; // index of this entry

        try {
            const id = await GuestRun({ vmRef: vm.ref, username, password, command: cmd });
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

    return (
        <div className="tab-body" style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
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
                    disabled={busy || !username}
                    autoComplete="off"
                />
                <button className="btn-primary" onClick={handleRun} disabled={busy || !command.trim() || !username}>
                    {busy ? 'Running…' : 'Run'}
                </button>
            </div>
        </div>
    );
}
