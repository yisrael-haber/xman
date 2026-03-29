import { useState, useEffect, useRef } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { VMGet, VMList } from '../../../../wailsjs/go/manager/Manager';
import VMBrowser from './VMBrowser';
import VMInfoTab from './VMInfoTab';
import FileTransferTab from './FileTransferTab';
import SnapshotsTab from './SnapshotsTab';
import GuestExecTab from './GuestExecTab';
import RemoteInstallTab from './RemoteInstallTab';
import DeploySSHKeyTab from './DeploySSHKeyTab';
type TabID = 'info' | 'filetransfer' | 'snapshots' | 'exec' | 'install' | 'deploykey';

const ALL_TABS: { id: TabID; label: string; requiresGuestOps?: boolean }[] = [
    { id: 'info',         label: 'VM Info'       },
    { id: 'snapshots',    label: 'Snapshots'     },
    { id: 'exec',         label: 'Run Command',  requiresGuestOps: true },
    { id: 'filetransfer', label: 'File Transfer', requiresGuestOps: true },
    { id: 'install',      label: 'Install',       requiresGuestOps: true },
    { id: 'deploykey',    label: 'Deploy SSH Key' },
];

interface Props {
    onJobStarted: (id: string, targetName?: string) => void;
    toolsInstall: boolean;
    guestOps: boolean;
    backendType: string;
}

function formatPowerState(state: string): string {
    switch (state) {
        case 'poweredOn':
            return 'Powered On';
        case 'poweredOff':
            return 'Powered Off';
        case 'suspended':
            return 'Suspended';
        default:
            return state || 'Unknown';
    }
}

function formatToolsStatus(state: string): string {
    switch (state) {
        case 'toolsOk':
            return 'Tools ready';
        case 'toolsOld':
            return 'Tools outdated';
        case 'toolsNotRunning':
            return 'Tools not running';
        case 'toolsNotInstalled':
            return 'Tools not installed';
        default:
            return state || 'Tools unknown';
    }
}

export default function VMPanel({ onJobStarted, toolsInstall, guestOps, backendType }: Props) {
    const [vms,      setVms]      = useState<manager.VMInfo[]>([]);
    const [selected, setSelected] = useState<manager.VMInfo | null>(null);
    const [loading,  setLoading]  = useState(false);
    const [error,    setError]    = useState('');
    const [activeTab, setActiveTab] = useState<TabID>('info');
    const refreshing = useRef(false);
    const queuedRefresh = useRef<boolean | null>(null);
    const selectedRefreshRef = useRef(0);

    async function loadVMs(silent = false): Promise<void> {
        if (refreshing.current) {
            queuedRefresh.current = queuedRefresh.current === null
                ? silent
                : queuedRefresh.current && silent;
            return;
        }

        refreshing.current = true;
        try {
            let nextSilent = silent;

            for (;;) {
                queuedRefresh.current = null;
                if (!nextSilent) setLoading(true);
                setError('');

                try {
                    const list = await VMList();
                    let selectedRefForRefresh = '';
                    let selectedPowerStateForRefresh = '';
                    setVms(list ?? []);
                    setSelected(prev => {
                        if (!prev) return null;
                        const nextSelected = list.find(v => v.ref === prev.ref) ?? prev;
                        selectedRefForRefresh = nextSelected.ref;
                        selectedPowerStateForRefresh = nextSelected.powerState;
                        return nextSelected;
                    });
                    if (selectedPowerStateForRefresh === 'poweredOn' && selectedRefForRefresh) {
                        void refreshSelectedVM(selectedRefForRefresh);
                    }
                } catch (e: any) {
                    setError(String(e));
                } finally {
                    if (!nextSilent) setLoading(false);
                }

                if (queuedRefresh.current === null) break;
                nextSilent = queuedRefresh.current;
            }
        } finally {
            refreshing.current = false;
        }
    }

    async function refreshSelectedVM(vmRef: string) {
        const token = ++selectedRefreshRef.current;
        try {
            const vm = await VMGet(vmRef);
            if (selectedRefreshRef.current !== token) return;
            setSelected(prev => prev?.ref === vmRef ? {
                ...vm,
                pathSegments: vm.pathSegments?.length ? vm.pathSegments : prev.pathSegments,
                displayPath: vm.displayPath || prev.displayPath,
            } : prev);
        } catch {
            // Keep the lighter list data if the targeted detail refresh fails.
        }
    }

    useEffect(() => {
        void loadVMs();
        const id = setInterval(() => loadVMs(true), 5_000);
        return () => clearInterval(id);
    }, []);

    useEffect(() => {
        if (!selected?.ref || selected.powerState !== 'poweredOn') return;
        void refreshSelectedVM(selected.ref);
    }, [selected?.ref, selected?.powerState]);

    return (
        <div className="vm-panel">
            <VMBrowser
                vms={vms}
                selected={selected}
                loading={loading}
                error={error}
                onSelect={setSelected}
                onRefresh={loadVMs}
            />

            <div className="vm-detail">
                {!selected ? (
                    <div className="vm-placeholder">Select a VM to get started.</div>
                ) : (
                    <>
                        <div className="vm-detail-header">
                            <div className="vm-detail-header-main">
                                <span className="vm-detail-eyebrow">Selected VM</span>
                                <div className="vm-detail-title-row">
                                    <h2 className="vm-detail-title">{selected.name}</h2>
                                    <span className={`badge badge--${selected.powerState === 'poweredOn' ? 'green' : selected.powerState === 'suspended' ? 'yellow' : 'gray'}`}>
                                        {formatPowerState(selected.powerState)}
                                    </span>
                                </div>
                                {selected.displayPath && (
                                    <div className="vm-detail-path" title={selected.displayPath}>
                                        {selected.displayPath}
                                    </div>
                                )}
                                <div className="vm-detail-meta">
                                    <span>{selected.guestOS || 'Guest OS unavailable'}</span>
                                    <span>{selected.ipAddress || 'No IP reported yet'}</span>
                                    <span>{formatToolsStatus(selected.toolsStatus)}</span>
                                </div>
                            </div>
                        </div>

                        <div className="tab-bar">
                            {ALL_TABS.filter(t => !t.requiresGuestOps || guestOps).map(tab => (
                                <button
                                    key={tab.id}
                                    className={`tab ${activeTab === tab.id ? 'tab--active' : ''}`}
                                    onClick={() => setActiveTab(tab.id)}
                                >
                                    {tab.label}
                                </button>
                            ))}
                        </div>

                        <div className="tab-content">
                            {activeTab === 'info' && (
                                <VMInfoTab
                                    vm={selected}
                                    onRefresh={loadVMs}
                                    onJobStarted={onJobStarted}
                                    toolsInstall={toolsInstall}
                                    backendType={backendType}
                                />
                            )}
                            {activeTab === 'snapshots' && (
                                <SnapshotsTab vm={selected} onJobStarted={onJobStarted} backendType={backendType} />
                            )}
                            {activeTab === 'exec' && (
                                <GuestExecTab vm={selected} onJobStarted={onJobStarted} />
                            )}
                            {activeTab === 'filetransfer' && (
                                <FileTransferTab vm={selected} onJobStarted={onJobStarted} />
                            )}
                            {activeTab === 'install' && (
                                <RemoteInstallTab vm={selected} onJobStarted={onJobStarted} />
                            )}
                            {activeTab === 'deploykey' && (
                                <DeploySSHKeyTab vm={selected} onJobStarted={onJobStarted} />
                            )}
                        </div>
                    </>
                )}
            </div>
        </div>
    );
}
