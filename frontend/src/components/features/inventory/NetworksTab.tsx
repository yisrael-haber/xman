import { manager } from '../../../../wailsjs/go/models';

interface Props {
    switches: manager.SwitchInfo[];
}

export default function NetworksTab({ switches }: Props) {
    if (switches.length === 0) {
        return <p className="vm-browser-empty">No networks found.</p>;
    }

    return (
        <>
        {switches.map((sw, i) => (
            <div key={i} className="inventory-card">
                <div className="inventory-card-header">
                    <span className="inventory-card-name">{sw.name}</span>
                    <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                        <span className="badge badge--gray">{sw.type}</span>
                        {sw.mtu > 0 && <span className="badge badge--gray">MTU {sw.mtu}</span>}
                        {sw.uplinks && sw.uplinks.length > 0 && (
                            <span className="badge badge--gray">{sw.uplinks.length} uplink{sw.uplinks.length !== 1 ? 's' : ''}</span>
                        )}
                    </div>
                </div>
                <div className="inventory-card-stats">
                    <div className="inventory-stat">
                        <span className="inventory-stat-label">Hosts</span>
                        <span className="inventory-stat-detail">
                            {sw.hosts && sw.hosts.length > 0 ? sw.hosts.join(', ') : '—'}
                        </span>
                    </div>
                    <div className="inventory-stat">
                        <span className="inventory-stat-label">Uplinks</span>
                        <span className="inventory-stat-detail">
                            {sw.uplinks && sw.uplinks.length > 0 ? sw.uplinks.join(', ') : '—'}
                        </span>
                    </div>
                </div>
                {sw.portGroups && sw.portGroups.length > 0 && (
                    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8rem', marginTop: '0.5rem' }}>
                        <thead>
                            <tr style={{ textAlign: 'left', opacity: 0.6 }}>
                                <th style={{ padding: '0.2rem 0.4rem', fontWeight: 500 }}>Name</th>
                                <th style={{ padding: '0.2rem 0.4rem', fontWeight: 500 }}>VLAN</th>
                                <th style={{ padding: '0.2rem 0.4rem', fontWeight: 500 }}>Hosts</th>
                                <th style={{ padding: '0.2rem 0.4rem', fontWeight: 500 }}>VMs</th>
                            </tr>
                        </thead>
                        <tbody>
                            {sw.portGroups.map((pg: manager.PortGroupInfo, j: number) => (
                                <tr key={j}>
                                    <td style={{ padding: '0.2rem 0.4rem' }}>{pg.name}</td>
                                    <td style={{ padding: '0.2rem 0.4rem' }}>{pg.vlan}</td>
                                    <td style={{ padding: '0.2rem 0.4rem' }}>
                                        {pg.hosts && pg.hosts.length > 0 ? pg.hosts.join(', ') : '—'}
                                    </td>
                                    <td style={{ padding: '0.2rem 0.4rem' }}>{pg.vmCount}</td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                )}
            </div>
        ))}
        </>
    );
}
