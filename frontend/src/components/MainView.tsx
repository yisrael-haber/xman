import { useState } from 'react';
import { Disconnect } from '../../wailsjs/go/main/App';
import { config } from '../../wailsjs/go/models';
import Sidebar, { FeatureID } from './Sidebar';
import JobsBar from './JobsBar';
import VMPanel from './features/vms/VMPanel';
import InventoryPanel from './features/inventory/InventoryPanel';
import { useJobs } from '../hooks/useJobs';

interface Props {
    info: config.ConnectionInfo;
    onDisconnect: () => void;
}

export default function MainView({ info, onDisconnect }: Props) {
    const [activeFeature, setActiveFeature] = useState<FeatureID>('vms');
    const { jobs, trackJob, dismiss } = useJobs();

    async function handleDisconnect() {
        await Disconnect();
        onDisconnect();
    }

    return (
        <div className="main-layout">
            <header className="main-header">
                <span className="main-header-title">xman</span>
                <div className="main-header-right">
                    <span className="connected-host">⬤ {info.displayName}</span>
                    <button className="disconnect-btn" onClick={handleDisconnect}>Disconnect</button>
                </div>
            </header>

            <div className="app-body">
                <Sidebar active={activeFeature} onChange={setActiveFeature} showInventory={info.inventory} />

                <div className="feature-content">
                    {activeFeature === 'vms' && (
                        <VMPanel onJobStarted={trackJob} toolsInstall={info.toolsInstall} />
                    )}
                    {activeFeature === 'inventory' && info.inventory && (
                        <InventoryPanel />
                    )}
                </div>
            </div>

            <JobsBar jobs={jobs} onDismiss={dismiss} />
        </div>
    );
}
