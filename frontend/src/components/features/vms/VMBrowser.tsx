import { manager } from '../../../../wailsjs/go/models';

interface Props {
    vms: manager.VMInfo[];
    selected: manager.VMInfo | null;
    loading: boolean;
    error: string;
    onSelect: (vm: manager.VMInfo) => void;
    onRefresh: () => Promise<void>;
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

function PowerDot({ state }: { state: string }) {
    const on = state === 'poweredOn';
    const suspended = state === 'suspended';
    return (
        <span
            className="power-dot"
            style={{ background: on ? '#4caf7d' : suspended ? '#e8a840' : '#445566' }}
            title={state}
        />
    );
}

export default function VMBrowser({ vms, selected, loading, error, onSelect, onRefresh }: Props) {
    return (
        <div className="vm-browser">
            <div className="vm-browser-header">
                <span className="vm-browser-title">Virtual Machines</span>
                <button className="icon-btn" onClick={() => void onRefresh()} title="Refresh" disabled={loading}>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="23 4 23 10 17 10"/>
                        <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
                    </svg>
                </button>
            </div>

            {error && <p className="vm-browser-error">{error}</p>}

            {loading && vms.length === 0 && (
                <p className="vm-browser-empty">Loading...</p>
            )}

            {!loading && vms.length === 0 && !error && (
                <p className="vm-browser-empty">No VMs found.</p>
            )}

            <ul className="vm-list">
                {vms.map(vm => {
                    const toolsOk = vm.toolsStatus === 'toolsOk' || vm.toolsStatus === 'toolsOld';
                    return (
                        <li
                            key={vm.ref}
                            className={`vm-item ${selected?.ref === vm.ref ? 'vm-item--active' : ''}`}
                            onClick={() => onSelect(vm)}
                        >
                            <PowerDot state={vm.powerState} />
                            <div className="vm-item-body">
                                <span className={`vm-name ${!toolsOk ? 'vm-name--no-tools' : ''}`} title={vm.name}>
                                    {vm.name}
                                </span>
                                <div className="vm-item-meta">
                                    <span className={`vm-state-chip vm-state-chip--${vm.powerState}`}>
                                        {formatPowerState(vm.powerState)}
                                    </span>
                                    <span
                                        className={`vm-meta-text ${!vm.ipAddress && !toolsOk ? 'vm-meta-text--warn' : ''}`}
                                        title={vm.ipAddress || formatToolsStatus(vm.toolsStatus)}
                                    >
                                        {vm.ipAddress || formatToolsStatus(vm.toolsStatus)}
                                    </span>
                                </div>
                            </div>
                        </li>
                    );
                })}
            </ul>
        </div>
    );
}
