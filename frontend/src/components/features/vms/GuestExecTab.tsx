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
    statusLabel: string;
    statusToneClass: string;
    transportLabel: string;
    sourceLabel: string;
    summary: string;
    metaLabel: string;
}

type ExecMode = 'raw' | 'script';

function isWindowsGuest(guestOS: string): boolean {
    return guestOS.toLowerCase().includes('win');
}

function summarizeCommandText(commandText: string, fallback: string): string {
    const lines = commandText
        .split('\n')
        .map(line => line.trim())
        .filter(Boolean);
    const summary = lines.find(line => !line.startsWith('#!')) || lines[0] || fallback;
    const collapsed = summary.split(/\s+/).join(' ');
    return collapsed.length > 96 ? `${collapsed.slice(0, 96).trimEnd()}…` : collapsed;
}

function formatRunClock(date: Date): string {
    return date.toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
    });
}

function formatRunDuration(ms: number): string {
    if (ms < 1_000) {
        return `${ms} ms`;
    }
    if (ms < 60_000) {
        return `${(ms / 1_000).toFixed(ms < 10_000 ? 1 : 0)}s`;
    }

    const totalSeconds = Math.round(ms / 1_000);
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
}

function formatRunMeta(startedAt: Date, endedAt?: Date): string {
    const parts = [`Started ${formatRunClock(startedAt)}`];
    if (endedAt) {
        parts.push(`Duration ${formatRunDuration(Math.max(0, endedAt.getTime() - startedAt.getTime()))}`);
    }
    return parts.join(' · ');
}

function describeResultStatus(
    status: OutputEntry['status'],
    finalMessage: string | undefined,
): Pick<OutputEntry, 'statusLabel' | 'statusToneClass'> {
    if (status === 'running') {
        return { statusLabel: 'Running', statusToneClass: 'badge--yellow' };
    }
    if (status === 'cancelled') {
        return { statusLabel: 'Cancelled', statusToneClass: 'badge--gray' };
    }
    if (status === 'failed') {
        return { statusLabel: 'Failed', statusToneClass: 'badge--red' };
    }
    if ((finalMessage || '').includes('non-zero exit status')) {
        return { statusLabel: 'Non-zero Exit', statusToneClass: 'badge--yellow' };
    }
    return { statusLabel: 'Completed', statusToneClass: 'badge--green' };
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
        const startedAt = new Date();
        const transportLabel = mode === 'ssh' ? 'SSH / Key' : 'Guest Ops';
        const sourceLabel = execMode === 'raw'
            ? 'Raw Command'
            : (selectedScript?.filename ? `Stored Script · ${selectedScript.filename}` : 'Stored Script');
        const summary = execMode === 'raw'
            ? summarizeCommandText(commandText, 'Command')
            : (selectedScript?.filename || summarizeCommandText(commandText, 'Stored Script'));
        const runningStatus = describeResultStatus('running', '');
        setResult({
            output: '',
            status: 'running',
            statusLabel: runningStatus.statusLabel,
            statusToneClass: runningStatus.statusToneClass,
            transportLabel,
            sourceLabel,
            summary,
            metaLabel: formatRunMeta(startedAt),
        });

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
                const finishedAt = new Date();
                const displayStatus = describeResultStatus(job.status, job?.message);
                const output = extractTerminalOutput(job);
                const nextResult: OutputEntry = {
                    output,
                    status: job.status,
                    statusLabel: displayStatus.statusLabel,
                    statusToneClass: displayStatus.statusToneClass,
                    transportLabel,
                    sourceLabel,
                    summary: typeof job?.label === 'string' && job.label.trim() ? job.label : summary,
                    metaLabel: formatRunMeta(startedAt, finishedAt),
                };
                if (job.status === 'done') {
                    setResult(nextResult);
                    setActiveJobId('');
                    setBusy(false);
                } else if (job.status === 'failed') {
                    setResult(nextResult);
                    setActiveJobId('');
                    setBusy(false);
                } else if (job.status === 'cancelled') {
                    setResult(nextResult);
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
                        {result ? (
                            <>
                                <div className="exec-shell-facts">
                                    <span className={`badge ${result.statusToneClass}`}>{result.statusLabel}</span>
                                    <span className="exec-shell-fact">{result.transportLabel}</span>
                                    <span className="exec-shell-fact">{result.sourceLabel}</span>
                                    <span className="exec-shell-fact">{result.metaLabel}</span>
                                </div>
                                <span className="exec-shell-subtitle" title={result.summary}>
                                    {result.summary}
                                </span>
                            </>
                        ) : (
                            <span className="exec-shell-subtitle">
                                Separate shell sessions. Each run replaces the previous output.
                            </span>
                        )}
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
