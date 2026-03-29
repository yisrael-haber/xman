import { useState } from 'react';
import SSHKeysManager from './SSHKeysManager';
import GuestCredentialsManager from './GuestCredentialsManager';

export default function SSHKeysPanel() {
    const [activeTab, setActiveTab] = useState<'sshkeys' | 'guestcredentials'>('sshkeys');

    return (
        <div className="vm-detail" style={{ width: '100%' }}>
            <div className="tab-bar">
                <button
                    className={`tab ${activeTab === 'sshkeys' ? 'tab--active' : ''}`}
                    onClick={() => setActiveTab('sshkeys')}
                >
                    SSH Keys
                </button>
                <button
                    className={`tab ${activeTab === 'guestcredentials' ? 'tab--active' : ''}`}
                    onClick={() => setActiveTab('guestcredentials')}
                >
                    Guest Credentials
                </button>
            </div>

            <div className="tab-content">
                {activeTab === 'sshkeys' ? <SSHKeysManager /> : <GuestCredentialsManager />}
            </div>
        </div>
    );
}
