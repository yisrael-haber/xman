import { useState } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { VMPowerOn, VMPowerOff, VMReset, VMSuspend, VMInstallTools } from '../../../../wailsjs/go/manager/Manager';
import { EventsOn } from '../../../../wailsjs/runtime/runtime';

interface Props {
    vm: manager.VMInfo;
    onRefresh: () => Promise<void>;
    onJobStarted: (id: string, targetName?: string) => void;
    toolsInstall: boolean;
    backendType: string;
}

const TOOLS_LABELS: Record<string, { label: string; ok: boolean }> = {
    toolsOk:           { label: 'OK',           ok: true  },
    toolsOld:          { label: 'Outdated',      ok: true  },
    toolsNotInstalled: { label: 'Not installed', ok: false },
    toolsNotRunning:   { label: 'Not running',   ok: false },
};

type PowerAction = 'on' | 'off' | 'reset' | 'suspend';

export default function VMInfoTab({ vm, onRefresh, onJobStarted, toolsInstall, backendType }: Props) {
    const [busyByVM, setBusyByVM] = useState<Record<string, PowerAction | null>>({});
    const [toolsBusyByVM, setToolsBusyByVM] = useState<Record<string, boolean>>({});
    const [errorByVM, setErrorByVM] = useState<Record<string, string>>({});

    const tools = TOOLS_LABELS[vm.toolsStatus] ?? { label: vm.toolsStatus, ok: false };
    const busy = busyByVM[vm.ref] ?? null;
    const toolsBusy = toolsBusyByVM[vm.ref] ?? false;
    const error = errorByVM[vm.ref] ?? '';
    const isOn = vm.powerState === 'poweredOn';
    const isOff = vm.powerState === 'poweredOff';
    const isSuspended = vm.powerState === 'suspended';

    const isWindowsGuest = vm.guestOS.toLowerCase().includes('win');
    const canInstallTools = toolsInstall && isWindowsGuest && isOn &&
        (vm.toolsStatus === 'toolsNotInstalled' || vm.toolsStatus === 'toolsOld' || vm.toolsStatus === 'toolsNotRunning');
    const toolsButtonLabel = vm.toolsStatus === 'toolsNotInstalled' ? 'Install VMware Tools' : 'Upgrade VMware Tools';
    const showGuestToolsNote = toolsInstall && isOn && !isWindowsGuest;
    const refLabel = backendType === 'workstation' ? 'VMX path' : 'MOR ref';

    async function runAction(action: PowerAction, fn: () => Promise<string>) {
        setErrorByVM(prev => ({ ...prev, [vm.ref]: '' }));
        setBusyByVM(prev => ({ ...prev, [vm.ref]: action }));
        try {
            const id = await fn();
            onJobStarted(id, vm.name || vm.ref);
            const unsub = EventsOn(`job:${id}`, (job: any) => {
                if (job.status === 'done') {
                    unsub();
                    void onRefresh().finally(() => {
                        setBusyByVM(prev => ({ ...prev, [vm.ref]: null }));
                    });
                } else if (job.status === 'failed' || job.status === 'cancelled') {
                    unsub();
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
            const unsub = EventsOn(`job:${jobId}`, (job: any) => {
                if (job.status === 'done' || job.status === 'failed' || job.status === 'cancelled') {
                    unsub();
                    setToolsBusyByVM(prev => ({ ...prev, [vm.ref]: false }));
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

    return (
        <div className="tab-body">
            <table className="info-table">
                <tbody>
                    <tr>
                        <th>Name</th>
                        <td>{vm.name}</td>
                    </tr>
                    <tr>
                        <th>Power state</th>
                        <td>
                            <span className={`badge badge--${isOn ? 'green' : isSuspended ? 'yellow' : 'gray'}`}>
                                {vm.powerState}
                            </span>
                        </td>
                    </tr>
                    <tr>
                        <th>Guest OS</th>
                        <td>{vm.guestOS || '—'}</td>
                    </tr>
                    <tr>
                        <th>IP address</th>
                        <td>{vm.ipAddress || '—'}</td>
                    </tr>
                    <tr>
                        <th>CPU</th>
                        <td>{vm.numCPU} vCPU{vm.numCPU !== 1 ? 's' : ''}</td>
                    </tr>
                    <tr>
                        <th>Memory</th>
                        <td>{vm.memoryMB >= 1024 ? `${(vm.memoryMB / 1024).toFixed(0)} GB` : `${vm.memoryMB} MB`}</td>
                    </tr>
                    <tr>
                        <th>VMware Tools</th>
                        <td>
                            <span className={`badge badge--${tools.ok ? 'green' : 'red'}`}>
                                {tools.label}
                            </span>
                            {canInstallTools && (
                                <button
                                    className="btn-secondary"
                                    disabled={toolsBusy}
                                    onClick={handleInstallTools}
                                    style={{ marginLeft: '0.5rem' }}
                                >
                                    {toolsBusy ? 'Submitting…' : toolsButtonLabel}
                                </button>
                            )}
                            {showGuestToolsNote && (
                                <div className="info-inline-note">
                                    For Linux and macOS guests, install `open-vm-tools` from the guest OS package manager.
                                </div>
                            )}
                        </td>
                    </tr>
                    <tr>
                        <th>{refLabel}</th>
                        <td className="mono">{vm.ref}</td>
                    </tr>
                </tbody>
            </table>

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

            {error && <p className="form-error">{error}</p>}
        </div>
    );
}
