import { useEffect, useState } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { OpenVMConsole } from '../../../../wailsjs/go/main/App';
import { VMConsoleInfo } from '../../../../wailsjs/go/manager/Manager';

interface Props {
    vm: manager.VMInfo;
}

export default function ConsoleTab({ vm }: Props) {
    const [openBusy, setOpenBusy] = useState(false);
    const [diagnosticsBusy, setDiagnosticsBusy] = useState(false);
    const [openError, setOpenError] = useState('');
    const [diagnosticsError, setDiagnosticsError] = useState('');
    const [info, setInfo] = useState<manager.ConsoleLaunchInfo | null>(null);

    async function loadDiagnostics() {
        setDiagnosticsBusy(true);
        setDiagnosticsError('');
        try {
            setInfo(await VMConsoleInfo(vm.ref));
        } catch (e: any) {
            setDiagnosticsError(String(e));
            setInfo(null);
        } finally {
            setDiagnosticsBusy(false);
        }
    }

    useEffect(() => {
        void loadDiagnostics();
    }, [vm.ref]);

    async function handleOpenConsole() {
        setOpenBusy(true);
        setOpenError('');
        try {
            await OpenVMConsole(vm.ref);
        } catch (e: any) {
            setOpenError(String(e));
        } finally {
            setOpenBusy(false);
        }
    }

    return (
        <div className="tab-body tab-body--fill">
            <div className="form-section console-panel">
                <div className="ssh-key-detail-header">
                    <div className="ssh-key-detail-title">Browser Console</div>
                    <div className="ssh-key-detail-subtitle">
                        Launch a fresh vCenter HTML5 console session for this VM.
                    </div>
                </div>

                <div className="notice notice--warn">
                    This opens in your default browser using the VMware console page served by vCenter. The diagnostics below show both the vCenter-reported console host and the current session endpoint so it&apos;s easier to spot DNS or reachability mismatches before launch.
                </div>

                <div className="console-actions">
                    <button className="btn-primary" disabled={openBusy} onClick={handleOpenConsole}>
                        {openBusy ? 'Opening…' : 'Open Console'}
                    </button>
                    <button className="btn-secondary" disabled={diagnosticsBusy} onClick={() => void loadDiagnostics()}>
                        {diagnosticsBusy ? 'Refreshing…' : 'Refresh Diagnostics'}
                    </button>
                    <span className="console-inline-note">
                        Each launch uses a fresh one-time session ticket.
                    </span>
                </div>

                <div className="console-details">
                    <div className="console-detail-row">
                        <span className="console-detail-label">VM</span>
                        <span>{vm.name}</span>
                    </div>
                    <div className="console-detail-row">
                        <span className="console-detail-label">Power state</span>
                        <span>{vm.powerState}</span>
                    </div>
                    <div className="console-detail-row">
                        <span className="console-detail-label">Path</span>
                        <span className="mono">{vm.displayPath || 'Inventory root'}</span>
                    </div>
                </div>

                {diagnosticsBusy && <div className="console-inline-note">Loading console diagnostics…</div>}

                {info && (
                    <>
                        {info.warnings?.length > 0 && (
                            <div className="console-warning-list">
                                {info.warnings.map((warning, index) => (
                                    <div key={`${warning}-${index}`} className="notice notice--warn">
                                        {warning}
                                    </div>
                                ))}
                            </div>
                        )}

                        <div className="console-details">
                            <div className="console-detail-row">
                                <span className="console-detail-label">vCenter URL</span>
                                <span className="mono">{info.vcenterUrl}</span>
                            </div>
                            <div className="console-detail-row">
                                <span className="console-detail-label">Connected host</span>
                                <span>{info.connectedHost || '—'}</span>
                            </div>
                            <div className="console-detail-row">
                                <span className="console-detail-label">Reported FQDN</span>
                                <span>{info.reportedFqdn || '—'}</span>
                            </div>
                            <div className="console-detail-row">
                                <span className="console-detail-label">Console host</span>
                                <span>{info.consoleHost || '—'}</span>
                            </div>
                            <div className="console-detail-row">
                                <span className="console-detail-label">Host source</span>
                                <span className="mono">{info.consoleHostSource || '—'}</span>
                            </div>
                            <div className="console-detail-row">
                                <span className="console-detail-label">Server GUID</span>
                                <span className="mono">{info.serverGuid || '—'}</span>
                            </div>
                            <div className="console-detail-row">
                                <span className="console-detail-label">Ticket</span>
                                <span className="mono">{info.ticketPreview || '—'}</span>
                            </div>
                            <div className="console-detail-row console-detail-row--stack">
                                <span className="console-detail-label">Redacted URL</span>
                                <span className="console-url-preview mono">{info.displayUrl || '—'}</span>
                            </div>
                        </div>

                        <div className="console-check-grid">
                            {[info.vcenterCheck, info.consoleHostCheck].map(check => (
                                <div key={check.name} className="console-check-card">
                                    <div className="console-check-header">
                                        <span className="console-check-title">{check.name}</span>
                                        <span className={`badge badge--${check.reachable ? 'green' : 'red'}`}>
                                            {check.reachable ? 'Reachable' : 'Not reachable'}
                                        </span>
                                    </div>
                                    <div className="console-check-address mono">
                                        {check.address || '—'}
                                    </div>
                                    {!check.reachable && check.error && (
                                        <div className="console-check-error">{check.error}</div>
                                    )}
                                </div>
                            ))}
                        </div>
                    </>
                )}

                {diagnosticsError && <p className="form-error">{diagnosticsError}</p>}
                {openError && <p className="form-error">{openError}</p>}
            </div>
        </div>
    );
}
