export function scriptKindFromFilename(filename: string): string {
    const trimmed = filename.trim().toLowerCase();
    if (trimmed.endsWith('.sh') || trimmed.endsWith('.bash') || trimmed.endsWith('.zsh') || trimmed.endsWith('.ksh')) {
        return 'posix';
    }
    if (trimmed.endsWith('.cmd') || trimmed.endsWith('.bat')) {
        return 'windows-batch';
    }
    if (trimmed.endsWith('.ps1')) {
        return 'powershell';
    }
    return 'generic';
}

export function scriptKindLabel(kind: string): string {
    switch (kind) {
    case 'posix':
        return 'POSIX shell';
    case 'windows-batch':
        return 'Windows batch';
    case 'powershell':
        return 'PowerShell';
    default:
        return 'Generic text';
    }
}

export function scriptCompatibility(kind: string, windowsGuest: boolean): { canRun: boolean; tone: 'info' | 'warn'; message: string } | null {
    switch (kind) {
    case 'posix':
        return windowsGuest
            ? { canRun: false, tone: 'warn', message: 'This script targets POSIX guests. Select a Linux or macOS VM, or use a generic text script instead.' }
            : { canRun: true, tone: 'info', message: 'This script will run as shell commands on the selected guest.' };
    case 'windows-batch':
        return windowsGuest
            ? { canRun: true, tone: 'info', message: 'This script will run as Windows batch commands on the selected guest.' }
            : { canRun: false, tone: 'warn', message: 'This script targets Windows guests. Select a Windows VM, or use a generic text script instead.' };
    case 'powershell':
        return { canRun: false, tone: 'warn', message: 'Stored PowerShell scripts are not supported yet in this tab. Use a .cmd/.bat file or a generic text script for now.' };
    default:
        return null;
    }
}
