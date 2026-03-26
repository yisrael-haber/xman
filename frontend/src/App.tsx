import { useState, useEffect } from 'react';
import { ConnectionInfo } from '../wailsjs/go/main/App';
import { config } from '../wailsjs/go/models';
import LoginView from './components/LoginView';
import MainView from './components/MainView';
import './App.css';

export default function App() {
    const [connInfo, setConnInfo] = useState<config.ConnectionInfo | null>(null);
    const [checking, setChecking] = useState(true);

    // On startup, check if a backend is already active (e.g. after hot reload)
    useEffect(() => {
        ConnectionInfo()
            .then(info => setConnInfo(info.displayName ? info : null))
            .finally(() => setChecking(false));
    }, []);

    if (checking) {
        return <div className="splash">Connecting...</div>;
    }

    if (connInfo) {
        return <MainView info={connInfo} onDisconnect={() => setConnInfo(null)} />;
    }

    return <LoginView onConnected={setConnInfo} />;
}
