import { useState } from 'react';
import { Disconnect } from '../../wailsjs/go/main/App';
import Sidebar, { FeatureID } from './Sidebar';
import JobsBar from './JobsBar';
import VMPanel from './features/vms/VMPanel';
import InventoryPanel from './features/inventory/InventoryPanel';
import { useJobs } from '../hooks/useJobs';

interface Props {
    host: string;
    onDisconnect: () => void;
}

export default function MainView({ host, onDisconnect }: Props) {
    const [activeFeature, setActiveFeature] = useState<FeatureID>('vms');
    const { jobs, trackJob, dismiss } = useJobs();

    async function handleDisconnect() {
        await Disconnect();
        onDisconnect();
    }

    return (
        <div className="main-layout">
            <header className="main-header">
                <span className="main-header-title">manosphere</span>
                <div className="main-header-right">
                    <span className="connected-host">⬤ {host}</span>
                    <button className="disconnect-btn" onClick={handleDisconnect}>Disconnect</button>
                </div>
            </header>

            <div className="app-body">
                <Sidebar active={activeFeature} onChange={setActiveFeature} />

                <div className="feature-content">
                    {activeFeature === 'vms' && (
                        <VMPanel onJobStarted={trackJob} />
                    )}
                    {activeFeature === 'inventory' && (
                        <InventoryPanel />
                    )}
                </div>
            </div>

            <JobsBar jobs={jobs} onDismiss={dismiss} />
        </div>
    );
}
