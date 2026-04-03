import { manager } from '../../../../wailsjs/go/models';

interface Props {
    switches: manager.SwitchInfo[];
}

function isWorkstationSwitchType(type: string): boolean {
    return type === 'bridged' || type === 'host-only' || type === 'nat' || type === 'custom';
}

export default function NetworksTab({ switches }: Props) {
    if (switches.length === 0) {
        return <p className="vm-browser-empty">No networks found.</p>;
    }

    return (
        <>
            {switches.map((sw, i) => {
                const workstationSwitch = isWorkstationSwitchType(sw.type);
                const primaryLabel = workstationSwitch ? 'Connected VMs' : 'Hosts';
                const secondaryLabel = workstationSwitch ? 'Addresses' : 'Uplinks';
                const secondaryBadge = workstationSwitch
                    ? `${sw.uplinks.length} address${sw.uplinks.length !== 1 ? 'es' : ''}`
                    : `${sw.uplinks.length} uplink${sw.uplinks.length !== 1 ? 's' : ''}`;

                return (
                    <div key={i} className="inventory-card">
                        <div className="inventory-card-header">
                            <span className="inventory-card-name">{sw.name}</span>
                            <div className="inventory-badge-row">
                                <span className="badge badge--gray">{sw.type}</span>
                                {sw.mtu > 0 && <span className="badge badge--gray">MTU {sw.mtu}</span>}
                                {sw.uplinks && sw.uplinks.length > 0 && (
                                    <span className="badge badge--gray">{secondaryBadge}</span>
                                )}
                            </div>
                        </div>
                        <div className="inventory-card-stats">
                            <div className="inventory-stat">
                                <span className="inventory-stat-label">{primaryLabel}</span>
                                <span className="inventory-stat-detail">
                                    {sw.hosts && sw.hosts.length > 0 ? sw.hosts.join(', ') : '—'}
                                </span>
                            </div>
                            <div className="inventory-stat">
                                <span className="inventory-stat-label">{secondaryLabel}</span>
                                <span className="inventory-stat-detail">
                                    {sw.uplinks && sw.uplinks.length > 0 ? sw.uplinks.join(', ') : '—'}
                                </span>
                            </div>
                        </div>
                        {sw.portGroups && sw.portGroups.length > 0 && (
                            <table className="inventory-portgroup-table">
                                <thead>
                                    <tr className="inventory-portgroup-head">
                                        <th className="inventory-portgroup-cell">Name</th>
                                        <th className="inventory-portgroup-cell">VLAN</th>
                                        <th className="inventory-portgroup-cell">Hosts</th>
                                        <th className="inventory-portgroup-cell">VMs</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {sw.portGroups.map((pg: manager.PortGroupInfo, j: number) => (
                                        <tr key={j} className="inventory-portgroup-row">
                                            <td className="inventory-portgroup-cell">{pg.name}</td>
                                            <td className="inventory-portgroup-cell">{pg.vlan}</td>
                                            <td className="inventory-portgroup-cell">
                                                {pg.hosts && pg.hosts.length > 0 ? pg.hosts.join(', ') : '—'}
                                            </td>
                                            <td className="inventory-portgroup-cell">{pg.vmCount}</td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        )}
                    </div>
                );
            })}
        </>
    );
}
