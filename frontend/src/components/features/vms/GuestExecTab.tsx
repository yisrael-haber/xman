import { useState, useRef, useEffect } from 'react';
import { config, manager } from '../../../../wailsjs/go/models';
import { GuestRun, SSHRun } from '../../../../wailsjs/go/manager/Manager';
import { JobCancel } from '../../../../wailsjs/go/jobs/Manager';
import { GetScript, LaunchInteractiveSSHSession, ListScripts } from '../../../../wailsjs/go/main/App';
import { ClipboardSetText } from '../../../../wailsjs/runtime/runtime';
import useTerminalJob from '../../../hooks/useTerminalJob';
import type { VMTransportState } from '../../../hooks/useVMTransport';
import { scriptCompatibility } from '../../../utils/scripts';
import { extractTerminalOutput } from '../../../utils/terminalJob';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string, targetName?: string) => void;
    transport: VMTransportState;
}

interface OutputEntry {
    output: string;
    status: 'running' | 'done' | 'failed' | 'cancelled';
}

type ExecMode = 'raw' | 'script';

function isWindowsGuest(guestOS: string): boolean {
    return guestOS.toLowerCase().includes('win');
}

export default function GuestExecTab({ vm, onJobStarted, transport }: Props) {
    const { mode, credentialLabel, sshHost, keyLabel, sshUser, vmPoweredOn } = transport;
    const [execMode, setExecMode] = useState<ExecMode>('raw');
    const [command, setCommand] = useState('');
    const [catalog, setCatalog] = useState<config.ScriptCatalog | null>(null);
    const [selectedScriptId, setSelectedScriptId] = useState('');
    const [selectedScript, setSelectedScript] = useState<config.StoredScript | null>(null);
    const [catalogLoading, setCatalogLoading] = useState(false);
    const [scriptLoading, setScriptLoading] = useState(false);
    const [result, setResult] = useState<OutputEntry | null>(null);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [catalogError, setCatalogError] = useState('');
    const [scriptError, setScriptError] = useState('');
    const [copied, setCopied] = useState(false);
    const [activeJobId, setActiveJobId] = useState('');
    const [launchingSession, setLaunchingSession] = useState(false);
    const outputRef = useRef<HTMLDivElement>(null);
    const loadTokenRef = useRef(0);
    const watchTerminalJob = useTerminalJob();
    const windowsGuest = isWindowsGuest(vm.guestOS || '');
    const sshReady = !!sshHost.trim() && !!keyLabel && !!sshUser;
    const transportReady = mode === 'vmware' ? (!!credentialLabel && vmPoweredOn) : sshReady;
    const compatibility = execMode === 'script' && selectedScript
        ? scriptCompatibility(selectedScript.kind, windowsGuest)
        : null;
    const compatibilityMessage = compatibility && !compatibility.canRun ? compatibility : null;
    const displayError = error || (execMode === 'script' ? (catalogError || scriptError) : '');
    const canRun = !busy && transportReady && (
        execMode === 'raw'
            ? !!command.trim()
            : (!!selectedScript?.content && !scriptLoading && (compatibility?.canRun ?? true))
    );

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
        setCatalogError('');
        setScriptError('');
        setCopied(false);
        setActiveJobId('');
        setLaunchingSession(false);
    }, [vm.ref]);

    useEffect(() => {
        if (execMode !== 'script') {
            return;
        }
        void loadCatalog();
    }, [execMode]);

    useEffect(() => {
        if (execMode !== 'script') {
            return;
        }

        const token = ++loadTokenRef.current;
        if (!selectedScriptId) {
            setSelectedScript(null);
            setScriptError('');
            setScriptLoading(false);
            return;
        }

        setScriptLoading(true);
        setScriptError('');
        void GetScript(selectedScriptId)
            .then(script => {
                if (loadTokenRef.current !== token) return;
                setSelectedScript(script);
            })
            .catch((nextError: unknown) => {
                if (loadTokenRef.current !== token) return;
                setSelectedScript(null);
                setScriptError(String(nextError));
            })
            .finally(() => {
                if (loadTokenRef.current !== token) return;
                setScriptLoading(false);
            });
    }, [execMode, selectedScriptId]);

    async function loadCatalog() {
        setCatalogLoading(true);
        setCatalogError('');
        try {
            const nextCatalog = await ListScripts();
            setCatalog(nextCatalog);

            const nextSelectedId = nextCatalog.scripts.some(script => script.id === selectedScriptId)
                ? selectedScriptId
                : (nextCatalog.scripts[0]?.id || '');
            setSelectedScriptId(nextSelectedId);
            if (!nextSelectedId) {
                setSelectedScript(null);
            }
        } catch (nextError: unknown) {
            setCatalogError(String(nextError));
            setCatalog(null);
            setSelectedScriptId('');
            setSelectedScript(null);
        } finally {
            setCatalogLoading(false);
        }
    }

    async function handleRun() {
        const rawCommand = command.trim();
        const scriptCommand = selectedScript?.content ?? '';
        const commandText = execMode === 'raw' ? rawCommand : scriptCommand;
        if (!commandText) return;

        setError('');
        setBusy(true);
        setResult({ output: '', status: 'running' });

        try {
            let id: string;
            if (mode === 'ssh') {
                id = await SSHRun({ host: sshHost, keyLabel, command: commandText });
            } else {
                id = await GuestRun({ vmRef: vm.ref, credentialLabel, username: '', password: '', command: commandText, guestOS: vm.guestOS });
            }
            setActiveJobId(id);
            onJobStarted(id, vm.name || vm.ref);

            watchTerminalJob(id, (job: any) => {
                if (job.status === 'done') {
                    setResult({ output: extractTerminalOutput(job), status: 'done' });
                    setActiveJobId('');
                    setBusy(false);
                } else if (job.status === 'failed') {
                    setResult({ output: extractTerminalOutput(job), status: 'failed' });
                    setActiveJobId('');
                    setBusy(false);
                } else if (job.status === 'cancelled') {
                    setResult({ output: extractTerminalOutput(job), status: 'cancelled' });
                    setActiveJobId('');
                    setBusy(false);
                }
            });

            if (execMode === 'raw') {
                setCommand('');
            }
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

    return (
        <div className="tab-body tab-body--fill tab-body--centered">
            <div className="exec-workspace">
                <div className="exec-control-shell">
                    <div className="mode-toggle exec-mode-toggle">
                        <button
                            className={`mode-btn ${execMode === 'raw' ? 'mode-btn--active' : ''}`}
                            onClick={() => setExecMode('raw')}
                        >
                            Raw Command
                        </button>
                        <button
                            className={`mode-btn ${execMode === 'script' ? 'mode-btn--active' : ''}`}
                            onClick={() => setExecMode('script')}
                        >
                            Stored Script
                        </button>
                    </div>

                    {execMode === 'raw' ? (
                        <div className="exec-input-row">
                            <span className="exec-prompt-sym">$</span>
                            <input
                                className="exec-input"
                                value={command}
                                onChange={e => setCommand(e.target.value)}
                                onKeyDown={handleKeyDown}
                                placeholder="command to run in guest"
                                disabled={busy || !transportReady}
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
                            {mode === 'ssh' && (
                                <button
                                    className="btn-secondary"
                                    onClick={() => void handleLaunchSession()}
                                    disabled={!sshReady || launchingSession || busy}
                                >
                                    {launchingSession ? 'Launching…' : 'Open Terminal'}
                                </button>
                            )}
                        </div>
                    ) : (
                        <>
                            <div className="exec-input-row">
                                <select
                                    className="exec-select"
                                    aria-label="Stored script"
                                    value={selectedScriptId}
                                    onChange={e => setSelectedScriptId(e.target.value)}
                                    disabled={catalogLoading || !(catalog?.scripts?.length) || busy}
                                >
                                    {catalog?.scripts?.length ? (
                                        catalog.scripts.map(script => (
                                            <option key={script.id} value={script.id}>
                                                {script.filename}
                                            </option>
                                        ))
                                    ) : (
                                        <option value="">{catalogLoading ? 'Loading scripts…' : 'No scripts available'}</option>
                                    )}
                                </select>
                                <button className="btn-primary" onClick={() => void handleRun()} disabled={!canRun}>
                                    {busy ? 'Running…' : 'Run Script'}
                                </button>
                                {busy && activeJobId && (
                                    <button className="btn-secondary" onClick={handleCancel}>
                                        Cancel
                                    </button>
                                )}
                                {mode === 'ssh' && (
                                    <button
                                        className="btn-secondary"
                                        onClick={() => void handleLaunchSession()}
                                        disabled={!sshReady || launchingSession || busy}
                                    >
                                        {launchingSession ? 'Launching…' : 'Open Terminal'}
                                    </button>
                                )}
                            </div>

                            {selectedScript && compatibilityMessage && (
                                <div className={`vm-transport-message vm-transport-message--${compatibilityMessage.tone}`}>
                                    {compatibilityMessage.message}
                                </div>
                        )}

                        {!catalogLoading && !(catalog?.scripts?.length) && !catalogError && (
                            <p className="exec-inline-note">Create stored scripts in Scripts.</p>
                        )}
                    </>
                )}

                    {displayError && <p className="form-error">{displayError}</p>}
                </div>

                <div className="exec-shell-header">
                    <div className="exec-shell-meta">
                        <span className="exec-shell-title">Output</span>
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
            </div>
        </div>
    );
}
