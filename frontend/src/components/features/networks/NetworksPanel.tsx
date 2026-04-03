import { useState, useEffect } from 'react';
import { InventoryNetworks } from '../../../../wailsjs/go/manager/API';
import NetworksTab from '../inventory/NetworksTab';
import { manager } from '../../../../wailsjs/go/models';

export default function NetworksPanel() {
    const [switches, setSwitches] = useState<manager.SwitchInfo[]>([]);
    const [loading,  setLoading]  = useState(false);
    const [error,    setError]    = useState('');

    async function load() {
        setLoading(true);
        setError('');
        try {
            const result = await InventoryNetworks();
            setSwitches(result?.switches ?? []);
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
                <button className="tab tab--active panel-tab-static" disabled>Networks</button>
                <button className="icon-btn panel-refresh-btn" onClick={load} disabled={loading} title="Refresh">
                    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="23 4 23 10 17 10"/>
                        <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
                    </svg>
                </button>
            </div>
            <div className="tab-content">
                <div className="tab-body">
                    {error   && <p className="form-error">{error}</p>}
                    {loading && <p className="vm-browser-empty">Loading…</p>}
                    {!loading && <NetworksTab switches={switches} />}
                </div>
            </div>
        </div>
    );
}
