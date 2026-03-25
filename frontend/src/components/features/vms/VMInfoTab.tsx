import { useState } from 'react';
import { vminfo } from '../../../../wailsjs/go/models';
import { VMPowerOn, VMPowerOff, VMReset, VMSuspend } from '../../../../wailsjs/go/vminfo/Binding';

interface Props {
    vm: vminfo.VMInfo;
    onRefresh: () => void;
}

const TOOLS_LABELS: Record<string, { label: string; ok: boolean }> = {
    toolsOk:           { label: 'OK',           ok: true  },
    toolsOld:          { label: 'Outdated',      ok: true  },
    toolsNotInstalled: { label: 'Not installed', ok: false },
    toolsNotRunning:   { label: 'Not running',   ok: false },
};

type PowerAction = 'on' | 'off' | 'reset' | 'suspend';

export default function VMInfoTab({ vm, onRefresh }: Props) {
    const [busy, setBusy] = useState<PowerAction | null>(null);
    const [error, setError] = useState('');

    const tools = TOOLS_LABELS[vm.toolsStatus] ?? { label: vm.toolsStatus, ok: false };
    const isOn = vm.powerState === 'poweredOn';
    const isOff = vm.powerState === 'poweredOff';
    const isSuspended = vm.powerState === 'suspended';

    async function runAction(action: PowerAction, fn: () => Promise<void>) {
        setError('');
        setBusy(action);
        try {
            await fn();
            onRefresh();
        } catch (e: any) {
            setError(String(e));
        } finally {
            setBusy(null);
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
                        </td>
                    </tr>
                    <tr>
                        <th>MOR ref</th>
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
