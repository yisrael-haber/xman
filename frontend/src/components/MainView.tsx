import { useState } from 'react';
import { Disconnect } from '../../wailsjs/go/main/App';
import { config } from '../../wailsjs/go/models';
import Sidebar, { FeatureID } from './Sidebar';
import JobsBar from './JobsBar';
import VMPanel from './features/vms/VMPanel';
import InventoryPanel from './features/inventory/InventoryPanel';
import NetworksPanel from './features/networks/NetworksPanel';
import SSHKeysPanel from './features/sshkeys/SSHKeysPanel';
import StoredScriptsManager from './features/sshkeys/StoredScriptsManager';
import { useJobs } from '../hooks/useJobs';

interface Props {
    info: config.ConnectionInfo;
    onDisconnect: () => void;
}

export default function MainView({ info, onDisconnect }: Props) {
    const [activeFeature, setActiveFeature] = useState<FeatureID>('vms');
    const { jobs, trackJob, watchTerminalJob, dismiss, cancel } = useJobs();

    async function handleDisconnect() {
        await Disconnect();
        onDisconnect();
    }

    return (
        <div className="main-layout">
            <header className="main-header">
                <span className="main-header-title">xman</span>
                <div className="main-header-right">
                    <span className="connected-host">
                        <span className="connected-host-dot" />
                        <span className="connected-host-label">{info.displayName}</span>
                    </span>
                    <button className="disconnect-btn" onClick={handleDisconnect}>Disconnect</button>
                </div>
            </header>

            <div className="app-body">
                <Sidebar active={activeFeature} onChange={setActiveFeature} showInventory={info.inventory} />

                <div className="feature-content">
                    {activeFeature === 'vms' && (
                        <VMPanel
                            onJobStarted={trackJob}
                            watchJobTerminal={watchTerminalJob}
                            toolsInstall={info.toolsInstall}
                            guestOps={info.guestOps}
                            console={info.console}
                            backendType={info.backendType}
                        />
                    )}
                    {activeFeature === 'networks' && (
                        <NetworksPanel />
                    )}
                    {activeFeature === 'sshkeys' && (
                        <SSHKeysPanel />
                    )}
                    {activeFeature === 'scripts' && (
                        <div className="vm-detail panel-shell">
                            <StoredScriptsManager />
                        </div>
                    )}
                    {activeFeature === 'inventory' && info.inventory && (
                        <InventoryPanel />
                    )}
                </div>
            </div>

            <JobsBar jobs={jobs} onDismiss={dismiss} onCancel={cancel} />
        </div>
    );
}
