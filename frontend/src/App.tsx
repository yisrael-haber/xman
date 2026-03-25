import { useState, useEffect } from 'react';
import { ConnectionStatus } from '../wailsjs/go/main/App';
import LoginView from './components/LoginView';
import MainView from './components/MainView';
import './App.css';

export default function App() {
    const [connectedHost, setConnectedHost] = useState<string | null>(null);
    const [checking, setChecking] = useState(true);

    // On startup, check if a session is already active (e.g. after hot reload)
    useEffect(() => {
        ConnectionStatus()
            .then(host => setConnectedHost(host || null))
            .finally(() => setChecking(false));
    }, []);

    if (checking) {
        return <div className="splash">Connecting...</div>;
    }

    if (connectedHost) {
        return <MainView host={connectedHost} onDisconnect={() => setConnectedHost(null)} />;
    }

    return <LoginView onConnected={setConnectedHost} />;
}
