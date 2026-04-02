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

export function isToolsReady(state: string): boolean {
    return state === 'toolsOk' || state === 'toolsOld';
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

export function guestOpsWarmupMessage(toolsStatus: string): string {
    if (toolsStatus === 'toolsNotInstalled') {
        return 'Guest operations are not ready yet. For a fresh Windows VM, use Bootstrap Guest Ops in VM Info. If setup has already started inside the guest, you can still try now and xman will surface the real vSphere result.';
    }
    return 'VMware Tools are still starting or vSphere has not marked guest operations ready yet. You can try now, and xman will show the real vSphere result if readiness is still catching up.';
}
