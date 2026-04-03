import { useState, useEffect, useRef, type KeyboardEvent as ReactKeyboardEvent, type PointerEvent as ReactPointerEvent } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { VMGet, VMList } from '../../../../wailsjs/go/manager/API';
import VMBrowser from './VMBrowser';
import VMInfoTab from './VMInfoTab';
import FileTransferTab from './FileTransferTab';
import SnapshotsTab from './SnapshotsTab';
import GuestExecTab from './GuestExecTab';
import DeploySSHKeyTab from './DeploySSHKeyTab';
import ConsoleTab from './ConsoleTab';
import { formatGuestOpsStatus, formatPowerState } from '../../../utils/vmStatus';
import useVMTransport from '../../../hooks/useVMTransport';
import VMTransportControls from './VMTransportControls';
import type { TrackJobHandler, WatchTerminalJobHandler } from '../../../hooks/useJobs';
type TabID = 'info' | 'console' | 'filetransfer' | 'snapshots' | 'exec' | 'deploykey';

const ALL_TABS: { id: TabID; label: string; requiresGuestOps?: boolean; requiresConsole?: boolean }[] = [
    { id: 'info',         label: 'Info'          },
    { id: 'console',      label: 'Console',       requiresConsole: true },
    { id: 'snapshots',    label: 'Snapshots'     },
    { id: 'exec',         label: 'Run'           },
    { id: 'filetransfer', label: 'Files',         requiresGuestOps: true },
    { id: 'deploykey',    label: 'SSH Key'       },
];

const VM_BROWSER_MIN_WIDTH = 200;
const VM_BROWSER_DEFAULT_WIDTH = 236;
const VM_BROWSER_MAX_WIDTH = 520;
const VM_DETAIL_MIN_WIDTH = 360;
const VM_BROWSER_WIDTH_STORAGE_KEY = 'xman.vmBrowserWidth';

interface Props {
    onJobStarted: TrackJobHandler;
    watchJobTerminal: WatchTerminalJobHandler;
    toolsInstall: boolean;
    guestOps: boolean;
    console: boolean;
    backendType: string;
}

interface VMDetailProps {
    onJobStarted: TrackJobHandler;
    watchJobTerminal: WatchTerminalJobHandler;
    toolsInstall: boolean;
    backendType: string;
    vm: manager.VMInfo;
    activeTab: TabID;
    visibleTabs: { id: TabID; label: string; requiresGuestOps?: boolean; requiresConsole?: boolean }[];
    onRefresh: () => Promise<void>;
    onTabChange: (tab: TabID) => void;
}

function mergeVMInfo(base: manager.VMInfo, incoming: manager.VMInfo): manager.VMInfo {
    return manager.VMInfo.createFrom({
        ...base,
        ...incoming,
        pathSegments: incoming.pathSegments?.length ? incoming.pathSegments : base.pathSegments,
        displayPath: incoming.displayPath || base.displayPath,
        guestHostname: incoming.guestHostname || base.guestHostname,
        firmware: incoming.firmware || base.firmware,
        hardwareVersion: incoming.hardwareVersion || base.hardwareVersion,
        uuid: incoming.uuid || base.uuid,
        notes: incoming.notes || base.notes,
        vmxPath: incoming.vmxPath || base.vmxPath,
        hostName: incoming.hostName || base.hostName,
        datastoreNames: incoming.datastoreNames?.length ? incoming.datastoreNames : base.datastoreNames,
        networkAdapters: incoming.networkAdapters?.length ? incoming.networkAdapters : base.networkAdapters,
    });
}

function readStoredBrowserWidth(): number {
    if (typeof window === 'undefined') return VM_BROWSER_DEFAULT_WIDTH;

    const raw = Number(window.localStorage.getItem(VM_BROWSER_WIDTH_STORAGE_KEY));
    return Number.isFinite(raw) ? raw : VM_BROWSER_DEFAULT_WIDTH;
}

function isDocumentVisible(): boolean {
    if (typeof document === 'undefined') return true;
    return document.visibilityState !== 'hidden';
}

function clampBrowserWidth(width: number, panelWidth: number): number {
    const minWidth = panelWidth > 0
        ? Math.min(VM_BROWSER_MIN_WIDTH, Math.max(180, panelWidth - VM_DETAIL_MIN_WIDTH))
        : VM_BROWSER_MIN_WIDTH;
    const maxWidth = panelWidth > 0
        ? Math.max(minWidth, Math.min(VM_BROWSER_MAX_WIDTH, panelWidth - VM_DETAIL_MIN_WIDTH))
        : VM_BROWSER_MAX_WIDTH;

    return Math.min(Math.max(width, minWidth), maxWidth);
}

function VMDetail({ vm, activeTab, visibleTabs, onRefresh, onTabChange, onJobStarted, watchJobTerminal, toolsInstall, backendType }: VMDetailProps) {
    const transportEnabled = activeTab === 'exec' || activeTab === 'filetransfer' || activeTab === 'deploykey';
    const transport = useVMTransport(vm, transportEnabled);
    const showTransportControls = activeTab === 'exec' || activeTab === 'filetransfer';
    const headerMeta = [
        vm.guestOS || '',
        vm.ipAddress || '',
        formatGuestOpsStatus(vm.powerState, vm.guestOpsReady, vm.toolsStatus),
    ].filter(Boolean);

    return (
        <>
            <div className="vm-detail-header">
                <div className="vm-detail-header-main">
                    <div className="vm-detail-title-row">
                        <h2 className="vm-detail-title">{vm.name}</h2>
                        <span className={`badge badge--${vm.powerState === 'poweredOn' ? 'green' : vm.powerState === 'suspended' ? 'yellow' : 'gray'}`}>
                            {formatPowerState(vm.powerState)}
                        </span>
                    </div>
                    {vm.displayPath && (
                        <div className="vm-detail-path" title={vm.displayPath}>
                            {vm.displayPath}
                        </div>
                    )}
                    {headerMeta.length > 0 && (
                        <div className="vm-detail-meta">
                            {headerMeta.map(item => (
                                <span key={item}>{item}</span>
                            ))}
                        </div>
                    )}
                </div>
            </div>

            <div className="tab-bar">
                {visibleTabs.map(tab => (
                    <button
                        key={tab.id}
                        className={`tab ${activeTab === tab.id ? 'tab--active' : ''}`}
                        onClick={() => onTabChange(tab.id)}
                    >
                        {tab.label}
                    </button>
                ))}
            </div>

            <div className="tab-content">
                {showTransportControls && <VMTransportControls transport={transport} />}

                {activeTab === 'info' && (
                    <VMInfoTab
                        vm={vm}
                        onRefresh={onRefresh}
                        onJobStarted={onJobStarted}
                        watchJobTerminal={watchJobTerminal}
                        toolsInstall={toolsInstall}
                        backendType={backendType}
                    />
                )}
                {activeTab === 'console' && (
                    <ConsoleTab vm={vm} />
                )}
                {activeTab === 'snapshots' && (
                    <SnapshotsTab vm={vm} onJobStarted={onJobStarted} watchJobTerminal={watchJobTerminal} backendType={backendType} />
                )}
                {activeTab === 'exec' && (
                    <GuestExecTab vm={vm} onJobStarted={onJobStarted} watchJobTerminal={watchJobTerminal} transport={transport} />
                )}
                {activeTab === 'filetransfer' && (
                    <FileTransferTab vm={vm} onJobStarted={onJobStarted} transport={transport} />
                )}
                {activeTab === 'deploykey' && (
                    <DeploySSHKeyTab vm={vm} onJobStarted={onJobStarted} transport={transport} />
                )}
            </div>
        </>
    );
}

export default function VMPanel({ onJobStarted, watchJobTerminal, toolsInstall, guestOps, console, backendType }: Props) {
    const [vms,      setVms]      = useState<manager.VMInfo[]>([]);
    const [selected, setSelected] = useState<manager.VMInfo | null>(null);
    const [loading,  setLoading]  = useState(false);
    const [error,    setError]    = useState('');
    const [activeTab, setActiveTab] = useState<TabID>('info');
    const [browserWidth, setBrowserWidth] = useState(() => readStoredBrowserWidth());
    const [panelWidth, setPanelWidth] = useState(0);
    const [isResizingBrowser, setIsResizingBrowser] = useState(false);
    const [documentVisible, setDocumentVisible] = useState(() => isDocumentVisible());
    const panelRef = useRef<HTMLDivElement | null>(null);
    const refreshing = useRef(false);
    const hasLoadedVMsRef = useRef(false);
    const queuedRefresh = useRef<boolean | null>(null);
    const selectedRefreshes = useRef(new Map<string, { token: symbol; promise: Promise<void> }>());
    const browserResizeStartRef = useRef<{ startX: number; startWidth: number } | null>(null);
    const effectiveBrowserWidth = clampBrowserWidth(browserWidth, panelWidth);
    const minBrowserWidth = clampBrowserWidth(VM_BROWSER_MIN_WIDTH, panelWidth);
    const maxBrowserWidth = clampBrowserWidth(VM_BROWSER_MAX_WIDTH, panelWidth);

    function updateBrowserWidth(nextWidth: number) {
        setBrowserWidth(clampBrowserWidth(nextWidth, panelWidth));
    }

    function resetBrowserWidth() {
        updateBrowserWidth(VM_BROWSER_DEFAULT_WIDTH);
    }

    function handleBrowserResizePointerDown(event: ReactPointerEvent<HTMLDivElement>) {
        if (event.button !== 0) return;

        browserResizeStartRef.current = {
            startX: event.clientX,
            startWidth: effectiveBrowserWidth,
        };
        setIsResizingBrowser(true);
        event.currentTarget.setPointerCapture(event.pointerId);
        event.preventDefault();
    }

    function handleBrowserResizePointerMove(event: ReactPointerEvent<HTMLDivElement>) {
        const resizeStart = browserResizeStartRef.current;
        if (!resizeStart) return;

        updateBrowserWidth(resizeStart.startWidth + (event.clientX - resizeStart.startX));
    }

    function finishBrowserResize(event: ReactPointerEvent<HTMLDivElement>) {
        if (event.currentTarget.hasPointerCapture(event.pointerId)) {
            event.currentTarget.releasePointerCapture(event.pointerId);
        }
        browserResizeStartRef.current = null;
        setIsResizingBrowser(false);
    }

    function handleBrowserResizeKeyDown(event: ReactKeyboardEvent<HTMLDivElement>) {
        switch (event.key) {
        case 'ArrowLeft':
            event.preventDefault();
            updateBrowserWidth(effectiveBrowserWidth - 24);
            break;
        case 'ArrowRight':
            event.preventDefault();
            updateBrowserWidth(effectiveBrowserWidth + 24);
            break;
        case 'Home':
            event.preventDefault();
            resetBrowserWidth();
            break;
        default:
            break;
        }
    }

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
                        const nextSelected = list.find(v => v.ref === prev.ref);
                        if (!nextSelected) return null;
                        const merged = mergeVMInfo(prev, nextSelected);
                        selectedRefForRefresh = merged.ref;
                        selectedPowerStateForRefresh = merged.powerState;
                        return merged;
                    });
                    if (backendType === 'workstation' && selectedPowerStateForRefresh === 'poweredOn' && selectedRefForRefresh) {
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
        const active = selectedRefreshes.current.get(vmRef);
        if (active) return active.promise;

        const token = Symbol(vmRef);
        const refreshPromise = (async () => {
            try {
                const vm = await VMGet(vmRef);
                setVms(prev => prev.map(entry => entry.ref === vmRef ? mergeVMInfo(entry, vm) : entry));
                setSelected(prev => prev?.ref === vmRef ? mergeVMInfo(prev, vm) : prev);
            } catch {
                // Keep the lighter list data if the targeted detail refresh fails.
            } finally {
                if (selectedRefreshes.current.get(vmRef)?.token === token) {
                    selectedRefreshes.current.delete(vmRef);
                }
            }
        })();

        selectedRefreshes.current.set(vmRef, { token, promise: refreshPromise });
        return refreshPromise;
    }

    useEffect(() => {
        if (typeof window !== 'undefined') {
            window.localStorage.setItem(VM_BROWSER_WIDTH_STORAGE_KEY, String(browserWidth));
        }
    }, [browserWidth]);

    useEffect(() => {
        if (typeof document === 'undefined') return;

        const handleVisibilityChange = () => {
            setDocumentVisible(isDocumentVisible());
        };

        handleVisibilityChange();
        document.addEventListener('visibilitychange', handleVisibilityChange);
        return () => document.removeEventListener('visibilitychange', handleVisibilityChange);
    }, []);

    useEffect(() => () => {
        selectedRefreshes.current.clear();
    }, []);

    useEffect(() => {
        const element = panelRef.current;
        if (!element) return;

        const updatePanelWidth = () => setPanelWidth(element.clientWidth);
        updatePanelWidth();

        if (typeof ResizeObserver === 'undefined') {
            window.addEventListener('resize', updatePanelWidth);
            return () => window.removeEventListener('resize', updatePanelWidth);
        }

        const observer = new ResizeObserver(entries => {
            const entry = entries[0];
            if (entry) {
                setPanelWidth(entry.contentRect.width);
            }
        });
        observer.observe(element);
        return () => observer.disconnect();
    }, []);

    useEffect(() => {
        const clamped = clampBrowserWidth(browserWidth, panelWidth);
        if (clamped !== browserWidth) {
            setBrowserWidth(clamped);
        }
    }, [browserWidth, panelWidth]);

    useEffect(() => {
        if (!isResizingBrowser) return;

        const previousUserSelect = document.body.style.userSelect;
        const previousCursor = document.body.style.cursor;
        document.body.style.userSelect = 'none';
        document.body.style.cursor = 'col-resize';

        return () => {
            document.body.style.userSelect = previousUserSelect;
            document.body.style.cursor = previousCursor;
        };
    }, [isResizingBrowser]);

    useEffect(() => {
        if (!documentVisible) return;

        const silent = hasLoadedVMsRef.current;
        hasLoadedVMsRef.current = true;
        void loadVMs(silent);
        const id = setInterval(() => loadVMs(true), 5_000);
        return () => clearInterval(id);
    }, [documentVisible]);

    useEffect(() => {
        if (!selected?.ref) return;
        void refreshSelectedVM(selected.ref);
    }, [selected?.ref]);

    useEffect(() => {
        if (backendType !== 'workstation') return;
        if (!selected?.ref || selected.powerState !== 'poweredOn') return;
        void refreshSelectedVM(selected.ref);
    }, [backendType, selected?.ref, selected?.powerState]);

    useEffect(() => {
        if (!documentVisible) return;
        if (!selected?.ref || selected.powerState !== 'poweredOn' || selected.guestOpsReady) return;
        const id = setInterval(() => {
            void refreshSelectedVM(selected.ref);
        }, 2_000);
        return () => clearInterval(id);
    }, [documentVisible, selected?.ref, selected?.powerState, selected?.guestOpsReady]);

    const visibleTabs = ALL_TABS.filter(tab =>
        (!tab.requiresGuestOps || guestOps) &&
        (!tab.requiresConsole || console),
    );

    useEffect(() => {
        if (!visibleTabs.some(tab => tab.id === activeTab)) {
            setActiveTab('info');
        }
    }, [activeTab, visibleTabs]);

    return (
        <div className="vm-panel" ref={panelRef}>
            <VMBrowser
                vms={vms}
                selected={selected}
                loading={loading}
                error={error}
                width={effectiveBrowserWidth}
                onSelect={setSelected}
                onRefresh={loadVMs}
            />

            <div
                className={`vm-browser-resizer ${isResizingBrowser ? 'vm-browser-resizer--active' : ''}`}
                role="separator"
                aria-label="Resize virtual machine list"
                aria-orientation="vertical"
                aria-valuemin={Math.round(minBrowserWidth)}
                aria-valuemax={Math.round(maxBrowserWidth)}
                aria-valuenow={Math.round(effectiveBrowserWidth)}
                tabIndex={0}
                title="Drag to resize the virtual machine list. Double-click to reset."
                onDoubleClick={resetBrowserWidth}
                onKeyDown={handleBrowserResizeKeyDown}
                onPointerDown={handleBrowserResizePointerDown}
                onPointerMove={handleBrowserResizePointerMove}
                onPointerUp={finishBrowserResize}
                onPointerCancel={finishBrowserResize}
            />

            <div className="vm-detail">
                {!selected ? (
                    <div className="vm-placeholder">Select a VM to get started.</div>
                ) : (
                    <VMDetail
                        vm={selected}
                        activeTab={activeTab}
                        visibleTabs={visibleTabs}
                        onRefresh={loadVMs}
                        onTabChange={setActiveTab}
                        onJobStarted={onJobStarted}
                        watchJobTerminal={watchJobTerminal}
                        toolsInstall={toolsInstall}
                        backendType={backendType}
                    />
                )}
            </div>
        </div>
    );
}
