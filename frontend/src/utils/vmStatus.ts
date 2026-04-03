export function formatPowerState(state: string): string {
    switch (state) {
        case 'poweredOn':
            return 'Powered On';
        case 'poweredOff':
            return 'Powered Off';
        case 'suspended':
            return 'Suspended';
        default:
            return state || 'Unknown';
    }
}

export function formatToolsStatus(state: string): string {
    switch (state) {
        case 'toolsOk':
            return 'Tools ready';
        case 'toolsOld':
            return 'Tools outdated';
        case 'toolsNotRunning':
            return 'Tools not running';
        case 'toolsNotInstalled':
            return 'Tools not installed';
        default:
            return state || 'Tools unknown';
    }
}

export function formatGuestOpsStatus(powerState: string, guestOpsReady: boolean, toolsStatus: string): string {
    if (powerState !== 'poweredOn') {
        return 'Guest ops unavailable';
    }
    if (guestOpsReady) {
        return 'Guest ops ready';
    }
    if (toolsStatus === 'toolsNotInstalled') {
        return 'Guest ops need VMware Tools';
    }
    return 'Guest ops starting';
}
