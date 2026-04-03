import { useState, useEffect } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { InventoryHosts, InventoryDatastores } from '../../../../wailsjs/go/manager/Manager';

type TabID = 'hosts' | 'datastores';

function UsageBar({ used, total }: { used: number; total: number }) {
    const pct = total > 0 ? Math.min(100, Math.round((used / total) * 100)) : 0;
    const color = pct > 85 ? '#e87a7a' : pct > 65 ? '#e8a840' : '#4a9eda';
    return (
        <div className="usage-bar-track">
            <div className="usage-bar-fill" style={{ width: `${pct}%`, background: color }} />
            <span className="usage-bar-label">{pct}%</span>
        </div>
    );
}

export default function InventoryPanel() {
    const [tab, setTab]               = useState<TabID>('hosts');
    const [hosts, setHosts]           = useState<manager.HostInfo[]>([]);
    const [datastores, setDatastores] = useState<manager.DatastoreInfo[]>([]);
    const [loading, setLoading]       = useState(false);
    const [error, setError]           = useState('');

    async function load() {
        setLoading(true);
        setError('');
        try {
            const [h, d] = await Promise.all([InventoryHosts(), InventoryDatastores()]);
            setHosts(h ?? []);
            setDatastores(d ?? []);
        } catch (e: any) {
            setError(String(e));
        } finally {
            setLoading(false);
        }
    }

    useEffect(() => { load(); }, []);

    return (
        <div className="vm-detail panel-shell">
            <div className="tab-bar">
                {(['hosts', 'datastores'] as TabID[]).map(id => (
                    <button key={id} className={`tab ${tab === id ? 'tab--active' : ''}`} onClick={() => setTab(id)}>
                        {id === 'hosts' ? 'ESXi Hosts' : 'Datastores'}
                    </button>
                ))}
                <button className="icon-btn panel-refresh-btn" onClick={load} disabled={loading} title="Refresh">
                    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="23 4 23 10 17 10"/>
                        <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
                    </svg>
                </button>
            </div>

            <div className="tab-content">
                <div className="tab-body">
                    {error && <p className="form-error">{error}</p>}
                    {loading && <p className="vm-browser-empty">Loading…</p>}

                    {tab === 'hosts' && !loading && (
                        <>
                        {hosts.length === 0 && !error && <p className="vm-browser-empty">No hosts found.</p>}
                        {hosts.map(h => (
                            <div key={h.ref} className="inventory-card">
                                <div className="inventory-card-header">
                                    <span className="inventory-card-name">{h.name || h.ref}</span>
                                    <div className="inventory-badge-row">
                                        <span className={`badge badge--${h.connectionState === 'connected' ? 'green' : 'red'}`}>
                                            {h.connectionState}
                                        </span>
                                        <span className={`badge badge--${h.powerState === 'poweredOn' ? 'green' : 'gray'}`}>
                                            {h.powerState}
                                        </span>
                                    </div>
                                </div>
                                <div className="inventory-card-stats">
                                    <div className="inventory-stat">
                                        <span className="inventory-stat-label">CPU</span>
                                        <UsageBar used={h.usedCPUMHz} total={h.totalCPUMHz} />
                                        <span className="inventory-stat-detail">{h.usedCPUMHz} / {h.totalCPUMHz} MHz</span>
                                    </div>
                                    <div className="inventory-stat">
                                        <span className="inventory-stat-label">Memory</span>
                                        <UsageBar used={h.usedMemoryMB} total={h.totalMemoryMB} />
                                        <span className="inventory-stat-detail">
                                            {(h.usedMemoryMB / 1024).toFixed(1)} / {(h.totalMemoryMB / 1024).toFixed(1)} GB
                                        </span>
                                    </div>
                                    <div className="inventory-stat">
                                        <span className="inventory-stat-label">VMs</span>
                                        <span className="inventory-stat-detail">{h.vmCount}</span>
                                    </div>
                                </div>
                            </div>
                        ))}
                        </>
                    )}

                    {tab === 'datastores' && !loading && (
                        <>
                        {datastores.length === 0 && !error && <p className="vm-browser-empty">No datastores found.</p>}
                        {datastores.map(ds => (
                            <div key={ds.ref} className="inventory-card">
                                <div className="inventory-card-header">
                                    <span className="inventory-card-name">{ds.name || ds.ref}</span>
                                    <div className="inventory-badge-row">
                                        <span className="badge badge--gray">{ds.type}</span>
                                        <span className={`badge badge--${ds.accessible ? 'green' : 'red'}`}>
                                            {ds.accessible ? 'accessible' : 'inaccessible'}
                                        </span>
                                    </div>
                                </div>
                                <div className="inventory-card-stats">
                                    <div className="inventory-stat inventory-stat--wide">
                                        <span className="inventory-stat-label">Storage</span>
                                        <UsageBar used={ds.capacityGB - ds.freeGB} total={ds.capacityGB} />
                                        <span className="inventory-stat-detail">
                                            {(ds.capacityGB - ds.freeGB).toFixed(1)} / {ds.capacityGB.toFixed(1)} GB used
                                        </span>
                                    </div>
                                    <div className="inventory-stat">
                                        <span className="inventory-stat-label">Free</span>
                                        <span className="inventory-stat-detail">{ds.freeGB.toFixed(1)} GB</span>
                                    </div>
                                </div>
                            </div>
                        ))}
                        </>
                    )}
                </div>
            </div>
        </div>
    );
}
