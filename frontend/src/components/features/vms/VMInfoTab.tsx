import { useEffect, useState } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { VMPowerOn, VMPowerOff, VMReset, VMSuspend, VMInstallTools, VMNetworkOptions, VMUpdateConfig, VMUpdateNetwork } from '../../../../wailsjs/go/manager/Manager';
import useTerminalJob from '../../../hooks/useTerminalJob';
import { formatPowerState } from '../../../utils/vmStatus';

interface Props {
    vm: manager.VMInfo;
    onRefresh: () => Promise<void>;
    onJobStarted: (id: string, targetName?: string) => void;
    toolsInstall: boolean;
    backendType: string;
}

type PowerAction = 'on' | 'off' | 'reset' | 'suspend';
type BadgeTone = 'green' | 'yellow' | 'gray' | 'red';
type MemoryUnit = 'mb' | 'gb';

interface ConfigDraft {
    name: string;
    notes: string;
    numCPU: string;
    memoryValue: string;
    memoryUnit: MemoryUnit;
    firmware: string;
}

interface NetworkDraft {
    networkId: string;
    connected: boolean;
}

const TOOLS_LABELS: Record<string, { label: string; ok: boolean }> = {
    toolsOk: { label: 'OK', ok: true },
    toolsOld: { label: 'Outdated', ok: true },
    toolsNotInstalled: { label: 'Not installed', ok: false },
    toolsNotRunning: { label: 'Not running', ok: false },
};

function formatMemory(memoryMB: number): string {
    if (memoryMB >= 1024) {
        return `${(memoryMB / 1024).toFixed(memoryMB % 1024 === 0 ? 0 : 1)} GB`;
    }
    return `${memoryMB} MB`;
}

function detailPieces(values: Array<string | null | undefined | false>): string {
    return values.filter(Boolean).join(' • ');
}

function normalizeFirmwareChoice(raw: string | undefined): string {
    switch ((raw ?? '').trim().toLowerCase()) {
    case '':
    case 'bios':
        return 'bios';
    case 'efi':
    case 'uefi':
        return 'efi';
    default:
        return 'bios';
    }
}

function preferredMemoryUnit(memoryMB: number): MemoryUnit {
    return memoryMB > 0 && memoryMB % 1024 === 0 ? 'gb' : 'mb';
}

function memoryValueForUnit(memoryMB: number, unit: MemoryUnit): string {
    if (unit === 'gb') {
        const roundedGB = Math.max(1, Math.round((memoryMB / 1024) * 100) / 100);
        return Number.isInteger(roundedGB) ? String(roundedGB) : String(roundedGB.toFixed(2)).replace(/\.?0+$/, '');
    }
    return String(Math.max(1, memoryMB));
}

function buildConfigDraft(vm: manager.VMInfo): ConfigDraft {
    const memoryUnit = preferredMemoryUnit(vm.memoryMB);
    return {
        name: vm.name,
        notes: vm.notes || '',
        numCPU: String(Math.max(1, vm.numCPU)),
        memoryValue: memoryValueForUnit(vm.memoryMB, memoryUnit),
        memoryUnit,
        firmware: normalizeFirmwareChoice(vm.firmware),
    };
}

function parseDraftMemoryMB(draft: ConfigDraft): number | null {
    const parsed = Number.parseFloat(draft.memoryValue);
    if (!Number.isFinite(parsed) || parsed <= 0) {
        return null;
    }

    const memoryMB = draft.memoryUnit === 'gb'
        ? Math.round(parsed * 1024)
        : Math.round(parsed);

    return memoryMB > 0 ? memoryMB : null;
}

function InfoRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
    return (
        <div className="vm-info-row">
            <div className="vm-info-row-label">{label}</div>
            <div className={`vm-info-row-value${mono ? ' mono' : ''}`}>{value}</div>
        </div>
    );
}

function adapterDetails(adapter: manager.VMNetworkAdapter): string {
    return detailPieces([
        adapter.network && adapter.label !== adapter.network ? adapter.label : '',
        adapter.networkType,
        adapter.macAddress ? `MAC ${adapter.macAddress}` : '',
        adapter.ipAddresses?.length ? adapter.ipAddresses.join(', ') : '',
    ]) || 'Adapter details unavailable';
}

function networkOptionLabel(option: manager.VMNetworkOption): string {
    return detailPieces([option.name, option.group, option.type]) || option.name;
}

export default function VMInfoTab({ vm, onRefresh, onJobStarted, toolsInstall, backendType }: Props) {
    const [busyByVM, setBusyByVM] = useState<Record<string, PowerAction | null>>({});
    const [toolsBusyByVM, setToolsBusyByVM] = useState<Record<string, boolean>>({});
    const [configBusy, setConfigBusy] = useState(false);
    const [configEditing, setConfigEditing] = useState(false);
    const [configDraft, setConfigDraft] = useState<ConfigDraft>(() => buildConfigDraft(vm));
    const [networkEditingAdapterId, setNetworkEditingAdapterId] = useState('');
    const [networkOptions, setNetworkOptions] = useState<manager.VMNetworkOption[]>([]);
    const [networkOptionsLoading, setNetworkOptionsLoading] = useState(false);
    const [networkBusy, setNetworkBusy] = useState(false);
    const [networkDraft, setNetworkDraft] = useState<NetworkDraft>({ networkId: '', connected: true });
    const [errorByVM, setErrorByVM] = useState<Record<string, string>>({});
    const watchTerminalJob = useTerminalJob();

    const tools = TOOLS_LABELS[vm.toolsStatus] ?? { label: vm.toolsStatus, ok: false };
    const busy = busyByVM[vm.ref] ?? null;
    const toolsBusy = toolsBusyByVM[vm.ref] ?? false;
    const error = errorByVM[vm.ref] ?? '';
    const isOn = vm.powerState === 'poweredOn';
    const isOff = vm.powerState === 'poweredOff';
    const isSuspended = vm.powerState === 'suspended';

    const isWindowsGuest = vm.guestOS.toLowerCase().includes('win');
    const canInstallTools = toolsInstall && isWindowsGuest && isOn &&
        (!vm.guestOpsReady || vm.toolsStatus === 'toolsOld' || vm.toolsStatus === 'toolsNotRunning' || vm.toolsStatus === 'toolsNotInstalled');
    const toolsButtonLabel = vm.guestOpsReady && vm.toolsStatus === 'toolsOld'
        ? 'Upgrade VMware Tools'
        : vm.toolsStatus === 'toolsNotInstalled'
            ? 'Bootstrap Guest Ops'
            : 'Repair VMware Tools';
    const showGuestToolsNote = toolsInstall && isOn && !isWindowsGuest;
    const refLabel = backendType === 'workstation' ? 'VMX path' : 'MOR ref';
    const guestOpsState = !isOn
        ? { label: 'Unavailable', tone: 'gray' as BadgeTone }
        : vm.guestOpsReady
            ? { label: 'Ready', tone: 'green' as BadgeTone }
            : vm.toolsStatus === 'toolsNotInstalled'
                ? { label: 'Needs Tools', tone: 'yellow' as BadgeTone }
                : { label: 'Starting', tone: 'yellow' as BadgeTone };
    const showGuestOpsWarmupNote = isOn && !vm.guestOpsReady;

    const placementRows = [
        vm.displayPath ? { label: 'Path', value: vm.displayPath } : null,
        vm.hostName ? { label: 'Host', value: vm.hostName } : null,
        vm.datastoreNames?.length ? { label: vm.datastoreNames.length > 1 ? 'Datastores' : 'Datastore', value: vm.datastoreNames.join(', ') } : null,
        { label: refLabel, value: backendType === 'workstation' ? (vm.vmxPath || vm.ref) : vm.ref, mono: true },
    ].filter(Boolean) as Array<{ label: string; value: string; mono?: boolean }>;

    const identityRows = [
        vm.guestHostname ? { label: 'Hostname', value: vm.guestHostname } : null,
        vm.firmware ? { label: 'Firmware', value: vm.firmware } : null,
        vm.hardwareVersion ? { label: 'Hardware', value: vm.hardwareVersion } : null,
        vm.uuid ? { label: 'UUID', value: vm.uuid, mono: true } : null,
    ].filter(Boolean) as Array<{ label: string; value: string; mono?: boolean }>;

    const overviewRows = [
        { label: 'Guest OS', value: vm.guestOS || 'Unknown guest' },
        vm.guestHostname ? { label: 'Hostname', value: vm.guestHostname } : null,
        { label: 'Primary IP', value: vm.ipAddress || 'No IP reported' },
        { label: 'Compute', value: `${vm.numCPU} vCPU${vm.numCPU !== 1 ? 's' : ''} • ${formatMemory(vm.memoryMB)}` },
    ].filter(Boolean) as Array<{ label: string; value: string }>;

    const memoryMB = parseDraftMemoryMB(configDraft);
    const numCPU = Number.parseInt(configDraft.numCPU, 10);
    const configNameEditable = backendType === 'vcenter' || isOff;
    const configNotesEditable = backendType === 'vcenter' || isOff;
    const configHardwareEditable = isOff;
    const configMemoryLabel = configDraft.memoryUnit === 'gb' ? 'Memory (GB)' : 'Memory (MB)';
    const configNotesSummary = vm.notes?.trim() ? vm.notes.trim() : 'No notes';
    const configStateMessage = backendType === 'workstation'
        ? (isOff
            ? 'This VM is powered off, so VMX-backed configuration is available now.'
            : 'Workstation stores configuration in the VMX file, so edits unlock only while the VM is powered off.')
        : (isOff
            ? 'Name, notes, CPU, memory, and firmware can be updated now.'
            : 'While the VM is running you can update the name and notes. CPU, memory, and firmware stay locked until power off.');
    const configNameValid = configDraft.name.trim() !== '';
    const configCPUValid = Number.isInteger(numCPU) && numCPU > 0;
    const configMemoryValid = memoryMB !== null;
    const hasConfigChanges = configDraft.name.trim() !== vm.name ||
        configDraft.notes.trim() !== (vm.notes || '').trim() ||
        numCPU !== vm.numCPU ||
        memoryMB !== vm.memoryMB ||
        normalizeFirmwareChoice(configDraft.firmware) !== normalizeFirmwareChoice(vm.firmware);
    const canSaveConfig = !configBusy && configNameValid && configCPUValid && configMemoryValid && hasConfigChanges;
    const activeNetworkAdapter = vm.networkAdapters?.find(adapter => adapter.id === networkEditingAdapterId) || null;
    const canEditNetwork = isOff;
    const networkStateMessage = isOff
        ? 'This VM is powered off, so NIC attachment changes are available now.'
        : 'NIC attachment changes currently require the VM to be powered off.';
    const hasNetworkChanges = !!activeNetworkAdapter &&
        (networkDraft.networkId !== (activeNetworkAdapter.networkId || '') || networkDraft.connected !== activeNetworkAdapter.connected);
    const canSaveNetwork = !!activeNetworkAdapter && canEditNetwork && !networkBusy && networkDraft.networkId !== '' && hasNetworkChanges;

    useEffect(() => {
        setConfigDraft(buildConfigDraft(vm));
        setConfigEditing(false);
        setConfigBusy(false);
        setNetworkEditingAdapterId('');
        setNetworkOptions([]);
        setNetworkOptionsLoading(false);
        setNetworkBusy(false);
        setNetworkDraft({ networkId: '', connected: true });
    }, [vm.ref]);

    useEffect(() => {
        if (!configEditing && !configBusy) {
            setConfigDraft(buildConfigDraft(vm));
        }
    }, [vm.name, vm.notes, vm.numCPU, vm.memoryMB, vm.firmware, configEditing, configBusy]);

    useEffect(() => {
        if (!activeNetworkAdapter || networkBusy) {
            return;
        }
        setNetworkDraft({
            networkId: activeNetworkAdapter.networkId || '',
            connected: activeNetworkAdapter.connected,
        });
    }, [activeNetworkAdapter, networkBusy]);

    async function runAction(action: PowerAction, fn: () => Promise<string>) {
        setErrorByVM(prev => ({ ...prev, [vm.ref]: '' }));
        setBusyByVM(prev => ({ ...prev, [vm.ref]: action }));
        try {
            const id = await fn();
            onJobStarted(id, vm.name || vm.ref);
            watchTerminalJob(id, (job: any) => {
                if (job.status === 'done') {
                    void onRefresh().finally(() => {
                        setBusyByVM(prev => ({ ...prev, [vm.ref]: null }));
                    });
                } else if (job.status === 'failed' || job.status === 'cancelled') {
                    setBusyByVM(prev => ({ ...prev, [vm.ref]: null }));
                    setErrorByVM(prev => ({ ...prev, [vm.ref]: job.error || 'Operation failed' }));
                }
            });
        } catch (e: any) {
            setErrorByVM(prev => ({ ...prev, [vm.ref]: String(e) }));
            setBusyByVM(prev => ({ ...prev, [vm.ref]: null }));
        }
    }

    async function handleInstallTools() {
        setErrorByVM(prev => ({ ...prev, [vm.ref]: '' }));
        setToolsBusyByVM(prev => ({ ...prev, [vm.ref]: true }));
        try {
            const jobId = await VMInstallTools(vm.ref);
            onJobStarted(jobId, vm.name || vm.ref);
            watchTerminalJob(jobId, (job: any) => {
                if (job.status === 'done' || job.status === 'failed' || job.status === 'cancelled') {
                    setToolsBusyByVM(prev => ({ ...prev, [vm.ref]: false }));
                    if (job.status === 'done') {
                        void onRefresh();
                    }
                    if (job.status === 'failed') {
                        setErrorByVM(prev => ({ ...prev, [vm.ref]: job.error || 'Tools installation failed' }));
                    }
                }
            });
        } catch (e: any) {
            setErrorByVM(prev => ({ ...prev, [vm.ref]: String(e) }));
            setToolsBusyByVM(prev => ({ ...prev, [vm.ref]: false }));
        }
    }

    async function handleSaveConfig() {
        if (!canSaveConfig || memoryMB === null || !configCPUValid) {
            return;
        }

        setErrorByVM(prev => ({ ...prev, [vm.ref]: '' }));
        setConfigBusy(true);
        try {
            const jobId = await VMUpdateConfig({
                vmRef: vm.ref,
                name: configDraft.name.trim(),
                notes: configDraft.notes.trim(),
                numCPU,
                memoryMB,
                firmware: normalizeFirmwareChoice(configDraft.firmware),
            });

            onJobStarted(jobId, vm.name || vm.ref);
            watchTerminalJob(jobId, (job: any) => {
                if (job.status === 'done') {
                    setConfigBusy(false);
                    setConfigEditing(false);
                    void onRefresh();
                } else if (job.status === 'failed' || job.status === 'cancelled') {
                    setConfigBusy(false);
                    setErrorByVM(prev => ({ ...prev, [vm.ref]: job.error || 'Configuration update failed' }));
                }
            });
        } catch (e: any) {
            setErrorByVM(prev => ({ ...prev, [vm.ref]: String(e) }));
            setConfigBusy(false);
        }
    }

    function handleCancelConfig() {
        setConfigDraft(buildConfigDraft(vm));
        setConfigEditing(false);
    }

    function updateConfigDraft<K extends keyof ConfigDraft>(key: K, value: ConfigDraft[K]) {
        setConfigDraft(prev => ({ ...prev, [key]: value }));
    }

    async function beginNetworkEdit(adapter: manager.VMNetworkAdapter) {
        setErrorByVM(prev => ({ ...prev, [vm.ref]: '' }));
        setNetworkEditingAdapterId(adapter.id);
        setNetworkDraft({
            networkId: adapter.networkId || '',
            connected: adapter.connected,
        });
        setNetworkOptionsLoading(true);
        try {
            const options = await VMNetworkOptions(vm.ref);
            setNetworkOptions(options);
            const hasCurrentNetwork = options.some(option => option.id === adapter.networkId);
            setNetworkDraft({
                networkId: hasCurrentNetwork ? adapter.networkId : (options[0]?.id || ''),
                connected: adapter.connected,
            });
        } catch (e: any) {
            setNetworkEditingAdapterId('');
            setNetworkOptions([]);
            setNetworkDraft({ networkId: '', connected: adapter.connected });
            setErrorByVM(prev => ({ ...prev, [vm.ref]: String(e) }));
        } finally {
            setNetworkOptionsLoading(false);
        }
    }

    function handleCancelNetwork() {
        setNetworkEditingAdapterId('');
        setNetworkOptions([]);
        setNetworkOptionsLoading(false);
        setNetworkBusy(false);
        setNetworkDraft({ networkId: '', connected: true });
    }

    function updateNetworkDraft<K extends keyof NetworkDraft>(key: K, value: NetworkDraft[K]) {
        setNetworkDraft(prev => ({ ...prev, [key]: value }));
    }

    async function handleSaveNetwork() {
        if (!activeNetworkAdapter || !canSaveNetwork) {
            return;
        }

        setErrorByVM(prev => ({ ...prev, [vm.ref]: '' }));
        setNetworkBusy(true);
        try {
            const jobId = await VMUpdateNetwork({
                vmRef: vm.ref,
                adapterId: activeNetworkAdapter.id,
                networkId: networkDraft.networkId,
                connected: networkDraft.connected,
            });

            onJobStarted(jobId, vm.name || vm.ref);
            watchTerminalJob(jobId, (job: any) => {
                if (job.status === 'done') {
                    setNetworkBusy(false);
                    handleCancelNetwork();
                    void onRefresh();
                } else if (job.status === 'failed' || job.status === 'cancelled') {
                    setNetworkBusy(false);
                    setErrorByVM(prev => ({ ...prev, [vm.ref]: job.error || 'Network update failed' }));
                }
            });
        } catch (e: any) {
            setErrorByVM(prev => ({ ...prev, [vm.ref]: String(e) }));
            setNetworkBusy(false);
        }
    }

    return (
        <div className="tab-body">
            <section className="vm-info-card vm-info-card--wide">
                <div className="vm-info-card-header">
                    <h3 className="vm-info-card-title">Power</h3>
                    <p className="vm-info-card-subtitle">The main VM power controls stay at the top for quick access.</p>
                </div>
                <div className="power-actions">
                    <button
                        className="btn-primary"
                        disabled={!!busy || isOn || isSuspended}
                        onClick={() => runAction('on', () => VMPowerOn(vm.ref))}
                    >
                        {busy === 'on' ? 'Powering on…' : 'Power On'}
                    </button>
                    <button
                        className="btn-secondary"
                        disabled={!!busy || isOff}
                        onClick={() => runAction('off', () => VMPowerOff(vm.ref))}
                    >
                        {busy === 'off' ? 'Powering off…' : 'Power Off'}
                    </button>
                    <button
                        className="btn-secondary"
                        disabled={!!busy || isOff}
                        onClick={() => runAction('suspend', () => VMSuspend(vm.ref))}
                    >
                        {busy === 'suspend' ? 'Suspending…' : 'Suspend'}
                    </button>
                    <button
                        className="btn-secondary"
                        disabled={!!busy || isOff}
                        onClick={() => runAction('reset', () => VMReset(vm.ref))}
                    >
                        {busy === 'reset' ? 'Resetting…' : 'Reset'}
                    </button>
                </div>
            </section>

            <section className="vm-info-card vm-info-card--wide">
                <div className="vm-info-card-header">
                    <div className="vm-info-card-header-row">
                        <div>
                            <h3 className="vm-info-card-title">Configuration</h3>
                            <p className="vm-info-card-subtitle">{configStateMessage}</p>
                        </div>
                        {!configEditing && (
                            <button className="btn-secondary" onClick={() => setConfigEditing(true)}>
                                Edit Configuration
                            </button>
                        )}
                    </div>
                </div>

                {!configEditing ? (
                    <div className="vm-info-cluster-grid">
                        <div className="vm-info-cluster">
                            <div className="vm-info-cluster-title">Editable fields</div>
                            <div className="vm-info-rows">
                                <InfoRow label="Name" value={vm.name} />
                                <InfoRow label="CPU" value={`${vm.numCPU} vCPU${vm.numCPU !== 1 ? 's' : ''}`} />
                                <InfoRow label="Memory" value={formatMemory(vm.memoryMB)} />
                                <InfoRow label="Firmware" value={vm.firmware || 'BIOS'} />
                            </div>
                        </div>
                        <div className="vm-info-cluster">
                            <div className="vm-info-cluster-title">Notes</div>
                            <div className="vm-info-notes-body">{configNotesSummary}</div>
                        </div>
                    </div>
                ) : (
                    <div className="vm-config-form">
                        <div className="vm-config-grid">
                            <div className="field">
                                <label htmlFor="vm-config-name">Name</label>
                                <input
                                    id="vm-config-name"
                                    value={configDraft.name}
                                    onChange={e => updateConfigDraft('name', e.target.value)}
                                    disabled={configBusy || !configNameEditable}
                                />
                                {!configNameEditable && (
                                    <div className="field-help">Rename is locked until this Workstation VM is powered off.</div>
                                )}
                            </div>

                            <div className="field">
                                <label htmlFor="vm-config-cpu">CPU</label>
                                <input
                                    id="vm-config-cpu"
                                    type="number"
                                    min="1"
                                    step="1"
                                    value={configDraft.numCPU}
                                    onChange={e => updateConfigDraft('numCPU', e.target.value)}
                                    disabled={configBusy || !configHardwareEditable}
                                />
                                {!configHardwareEditable && (
                                    <div className="field-help">CPU changes require the VM to be powered off.</div>
                                )}
                            </div>

                            <div className="field vm-config-memory-field">
                                <label htmlFor="vm-config-memory">Memory</label>
                                <div className="vm-config-memory-row">
                                    <input
                                        id="vm-config-memory"
                                        type="number"
                                        min="1"
                                        step={configDraft.memoryUnit === 'gb' ? '1' : '128'}
                                        value={configDraft.memoryValue}
                                        onChange={e => updateConfigDraft('memoryValue', e.target.value)}
                                        disabled={configBusy || !configHardwareEditable}
                                    />
                                    <select
                                        value={configDraft.memoryUnit}
                                        onChange={e => {
                                            const nextUnit = e.target.value as MemoryUnit;
                                            const nextMemoryMB = parseDraftMemoryMB(configDraft) ?? vm.memoryMB;
                                            updateConfigDraft('memoryUnit', nextUnit);
                                            updateConfigDraft('memoryValue', memoryValueForUnit(nextMemoryMB, nextUnit));
                                        }}
                                        disabled={configBusy || !configHardwareEditable}
                                    >
                                        <option value="mb">MB</option>
                                        <option value="gb">GB</option>
                                    </select>
                                </div>
                                {!configHardwareEditable && (
                                    <div className="field-help">Memory changes require the VM to be powered off.</div>
                                )}
                                {configHardwareEditable && !configMemoryValid && (
                                    <div className="field-help">Enter a valid positive memory amount.</div>
                                )}
                                {configHardwareEditable && configMemoryValid && (
                                    <div className="field-help">{configMemoryLabel}</div>
                                )}
                            </div>

                            <div className="field">
                                <label htmlFor="vm-config-firmware">Firmware</label>
                                <select
                                    id="vm-config-firmware"
                                    value={configDraft.firmware}
                                    onChange={e => updateConfigDraft('firmware', e.target.value)}
                                    disabled={configBusy || !configHardwareEditable}
                                >
                                    <option value="bios">BIOS</option>
                                    <option value="efi">UEFI</option>
                                </select>
                                {!configHardwareEditable && (
                                    <div className="field-help">Firmware changes require the VM to be powered off.</div>
                                )}
                            </div>
                        </div>

                        <div className="field">
                            <label htmlFor="vm-config-notes">Notes</label>
                            <textarea
                                id="vm-config-notes"
                                rows={4}
                                value={configDraft.notes}
                                onChange={e => updateConfigDraft('notes', e.target.value)}
                                disabled={configBusy || !configNotesEditable}
                                placeholder="Add VM notes or operational context"
                            />
                            {!configNotesEditable && (
                                <div className="field-help">Notes stay read-only until this Workstation VM is powered off.</div>
                            )}
                        </div>

                        <div className="vm-config-actions">
                            <button className="btn-primary" disabled={!canSaveConfig} onClick={() => void handleSaveConfig()}>
                                {configBusy ? 'Saving…' : 'Save Changes'}
                            </button>
                            <button className="btn-secondary" disabled={configBusy} onClick={handleCancelConfig}>
                                Cancel
                            </button>
                            {!hasConfigChanges && (
                                <span className="vm-config-meta">No unsaved changes.</span>
                            )}
                        </div>
                    </div>
                )}
            </section>

            <div className="vm-info-overview-grid">
                <section className="vm-info-card">
                    <div className="vm-info-card-header">
                        <h3 className="vm-info-card-title">Overview</h3>
                        <p className="vm-info-card-subtitle">The core guest and compute details you usually need first.</p>
                    </div>
                    <div className="vm-info-rows">
                        {overviewRows.map(row => (
                            <InfoRow key={row.label} label={row.label} value={row.value} />
                        ))}
                    </div>
                </section>

                <section className="vm-info-card">
                    <div className="vm-info-card-header">
                        <h3 className="vm-info-card-title">Status</h3>
                        <p className="vm-info-card-subtitle">Operational state, Tools health, and guest-ops readiness.</p>
                    </div>
                    <div className="vm-info-status-grid">
                        <div className="vm-info-status-item">
                            <div className="vm-info-status-label">Power</div>
                            <span className={`badge badge--${isOn ? 'green' : isSuspended ? 'yellow' : 'gray'}`}>
                                {formatPowerState(vm.powerState)}
                            </span>
                        </div>
                        <div className="vm-info-status-item">
                            <div className="vm-info-status-label">VMware Tools</div>
                            <span className={`badge badge--${tools.ok ? 'green' : 'red'}`}>{tools.label}</span>
                        </div>
                        <div className="vm-info-status-item">
                            <div className="vm-info-status-label">Guest Ops</div>
                            <span className={`badge badge--${guestOpsState.tone}`}>{guestOpsState.label}</span>
                        </div>
                    </div>
                    {(showGuestToolsNote || showGuestOpsWarmupNote) && (
                        <div className="vm-info-status-notes">
                            {showGuestToolsNote && (
                                <div className="vm-info-card-subtitle">
                                    For Linux and macOS guests, install open-vm-tools from the guest package manager.
                                </div>
                            )}
                            {showGuestOpsWarmupNote && (
                                <div className="vm-info-card-subtitle">
                                    Still warming up. You can try the Guest Ops tabs and get the real backend result.
                                </div>
                            )}
                        </div>
                    )}
                    {canInstallTools && (
                        <button
                            className="btn-secondary vm-info-inline-action"
                            disabled={toolsBusy}
                            onClick={handleInstallTools}
                        >
                            {toolsBusy ? 'Submitting…' : toolsButtonLabel}
                        </button>
                    )}
                </section>
            </div>

            <div className="vm-info-grid">
                <section className="vm-info-card vm-info-card--wide">
                    <div className="vm-info-card-header">
                        <h3 className="vm-info-card-title">Placement & Identity</h3>
                        <p className="vm-info-card-subtitle">Where this VM lives plus the hardware and guest identity around it.</p>
                    </div>
                    <div className="vm-info-cluster-grid">
                        <div className="vm-info-cluster">
                            <div className="vm-info-cluster-title">Placement</div>
                            <div className="vm-info-rows">
                                {placementRows.map(row => (
                                    <InfoRow key={row.label} label={row.label} value={row.value} mono={row.mono} />
                                ))}
                            </div>
                        </div>

                        {(identityRows.length > 0 || vm.notes) && (
                            <div className="vm-info-cluster">
                                <div className="vm-info-cluster-title">Identity</div>
                                {identityRows.length > 0 && (
                                    <div className="vm-info-rows">
                                        {identityRows.map(row => (
                                            <InfoRow key={row.label} label={row.label} value={row.value} mono={row.mono} />
                                        ))}
                                    </div>
                                )}
                                {vm.notes && (
                                    <div className="vm-info-notes">
                                        <div className="vm-info-row-label">Notes</div>
                                        <div className="vm-info-notes-body">{vm.notes}</div>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                </section>

                {vm.networkAdapters?.length ? (
                    <section className="vm-info-card">
                        <div className="vm-info-card-header">
                            <div className="vm-info-card-header-row">
                                <div>
                                    <h3 className="vm-info-card-title">Network</h3>
                                    <p className="vm-info-card-subtitle">Per-adapter attachment and connection state at a glance.</p>
                                </div>
                                <span className="vm-config-meta">{networkStateMessage}</span>
                            </div>
                        </div>
                        <div className="vm-network-list">
                            {vm.networkAdapters.map(adapter => (
                                <div
                                    key={adapter.id || `${adapter.label}-${adapter.macAddress || adapter.network || 'network'}`}
                                    className="vm-network-item"
                                >
                                    <div className="vm-network-topline">
                                        <div className="vm-network-name">{adapter.network || adapter.label}</div>
                                        <span className={`badge badge--${adapter.connected ? 'green' : 'gray'}`}>
                                            {adapter.connected ? 'Connected' : 'Disconnected'}
                                        </span>
                                    </div>
                                    <div className="vm-network-detail">{adapterDetails(adapter)}</div>
                                    {networkEditingAdapterId === adapter.id ? (
                                        <div className="vm-network-form">
                                            <div className="vm-config-grid">
                                                <div className="field">
                                                    <label htmlFor={`vm-network-target-${adapter.id}`}>Attached Network</label>
                                                    <select
                                                        id={`vm-network-target-${adapter.id}`}
                                                        value={networkDraft.networkId}
                                                        onChange={e => updateNetworkDraft('networkId', e.target.value)}
                                                        disabled={networkBusy || networkOptionsLoading}
                                                    >
                                                        {networkOptions.length === 0 ? (
                                                            <option value="">No networks available</option>
                                                        ) : (
                                                            networkOptions.map(option => (
                                                                <option key={option.id} value={option.id}>
                                                                    {networkOptionLabel(option)}
                                                                </option>
                                                            ))
                                                        )}
                                                    </select>
                                                    <div className="field-help">
                                                        {networkOptionsLoading ? 'Loading available networks…' : 'Pick the target network for this virtual NIC.'}
                                                    </div>
                                                </div>

                                                <label className="vm-network-checkbox">
                                                    <input
                                                        type="checkbox"
                                                        checked={networkDraft.connected}
                                                        onChange={e => updateNetworkDraft('connected', e.target.checked)}
                                                        disabled={networkBusy}
                                                    />
                                                    <span>Connect this adapter when the VM starts</span>
                                                </label>
                                            </div>

                                            <div className="vm-config-actions">
                                                <button className="btn-primary" disabled={!canSaveNetwork} onClick={() => void handleSaveNetwork()}>
                                                    {networkBusy ? 'Saving…' : 'Save Attachment'}
                                                </button>
                                                <button className="btn-secondary" disabled={networkBusy} onClick={handleCancelNetwork}>
                                                    Cancel
                                                </button>
                                                {!hasNetworkChanges && (
                                                    <span className="vm-config-meta">No unsaved changes.</span>
                                                )}
                                            </div>
                                        </div>
                                    ) : (
                                        <div className="vm-network-actions">
                                            <button
                                                className="btn-secondary"
                                                disabled={!canEditNetwork || networkBusy || !!networkEditingAdapterId}
                                                onClick={() => void beginNetworkEdit(adapter)}
                                            >
                                                Edit Attachment
                                            </button>
                                        </div>
                                    )}
                                </div>
                            ))}
                        </div>
                    </section>
                ) : null}
            </div>

            {error && <p className="form-error">{error}</p>}
        </div>
    );
}
