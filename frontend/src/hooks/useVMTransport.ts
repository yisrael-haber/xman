import { useEffect, useRef, useState } from 'react';
import { config, manager } from '../../wailsjs/go/models';
import { ListGuestCredentials, ListSSHKeys } from '../../wailsjs/go/main/App';

export type VMTransportMode = 'vmware' | 'ssh';

export default function useVMTransport(vm: manager.VMInfo, enabled = true) {
    const [mode, setMode] = useState<VMTransportMode>('vmware');
    const [credentialLabel, setCredentialLabel] = useState('');
    const [sshHost, setSshHostState] = useState(vm.ipAddress || '');
    const [keyLabel, setKeyLabel] = useState('');
    const [keys, setKeys] = useState<config.KeyMeta[]>([]);
    const [keysError, setKeysError] = useState('');
    const [credentials, setCredentials] = useState<config.GuestCredentialMeta[]>([]);
    const [credentialsError, setCredentialsError] = useState('');
    const previousVMRef = useRef(vm.ref);
    const sshHostTouchedRef = useRef(false);

    useEffect(() => {
        if (!enabled) {
            return;
        }

        let cancelled = false;

        ListSSHKeys()
            .then(list => {
                if (cancelled) return;
                setKeys(list ?? []);
                setKeysError('');
            })
            .catch((error: unknown) => {
                if (cancelled) return;
                setKeysError(String(error));
            });

        return () => {
            cancelled = true;
        };
    }, [enabled]);

    useEffect(() => {
        if (!enabled) {
            return;
        }

        let cancelled = false;

        ListGuestCredentials()
            .then(list => {
                if (cancelled) return;
                setCredentials(list ?? []);
                setCredentialsError('');
            })
            .catch((error: unknown) => {
                if (cancelled) return;
                setCredentialsError(String(error));
            });

        return () => {
            cancelled = true;
        };
    }, [enabled]);

    useEffect(() => {
        const switchingVM = previousVMRef.current !== vm.ref;
        previousVMRef.current = vm.ref;
        if (switchingVM) {
            sshHostTouchedRef.current = false;
        }
        if (!sshHostTouchedRef.current) {
            setSshHostState(vm.ipAddress || '');
        }
    }, [vm.ref, vm.ipAddress]);

    useEffect(() => {
        if (!keys.length) {
            setKeyLabel('');
            return;
        }
        if (!keyLabel || !keys.some(key => key.label === keyLabel)) {
            setKeyLabel(keys[0].label);
        }
    }, [keys, keyLabel]);

    useEffect(() => {
        if (!credentials.length) {
            setCredentialLabel('');
            return;
        }
        if (!credentialLabel || !credentials.some(credential => credential.label === credentialLabel)) {
            setCredentialLabel(credentials[0].label);
        }
    }, [credentials, credentialLabel]);

    const selectedKey = keys.find(key => key.label === keyLabel);
    const selectedKeyUser = selectedKey?.defaultUser?.trim() || '';
    const toolsStatus = vm.toolsStatus;
    const vmPoweredOn = vm.powerState === 'poweredOn';
    const guestOpsReady = !!vm.guestOpsReady;
    const vmwareReady = !!credentialLabel && vmPoweredOn && guestOpsReady;
    const vmwareCanAttempt = !!credentialLabel && vmPoweredOn && toolsStatus !== 'toolsNotInstalled';
    const sshReady = !!sshHost.trim() && !!keyLabel && !!selectedKeyUser;
    const transportReady = mode === 'vmware' ? vmwareCanAttempt : sshReady;

    let transportMessage = '';
    let transportMessageTone: 'info' | 'warn' | 'error' = 'info';

    if (mode === 'vmware') {
        if (credentialsError) {
            transportMessage = credentialsError;
            transportMessageTone = 'error';
        } else if (credentials.length === 0) {
            transportMessage = 'No guest credentials available.';
            transportMessageTone = 'warn';
        } else if (!vmPoweredOn) {
            transportMessage = 'Requires a powered-on VM.';
            transportMessageTone = 'warn';
        } else if (toolsStatus === 'toolsNotInstalled') {
            transportMessage = 'Requires VMware Tools.';
            transportMessageTone = 'warn';
        } else if (!guestOpsReady) {
            transportMessage = 'Guest Ops is still starting. You can try now and VMware will report if Tools is not ready yet.';
            transportMessageTone = 'info';
        }
    } else {
        if (keysError) {
            transportMessage = keysError;
            transportMessageTone = 'error';
        } else if (keys.length === 0) {
            transportMessage = 'No SSH keys available.';
            transportMessageTone = 'warn';
        } else if (selectedKey && !selectedKeyUser) {
            transportMessage = 'Selected key needs a default user.';
            transportMessageTone = 'warn';
        }
    }

    function setSshHost(nextHost: string) {
        sshHostTouchedRef.current = nextHost.trim() !== (vm.ipAddress || '').trim();
        setSshHostState(nextHost);
    }

    return {
        mode,
        setMode,
        credentialLabel,
        setCredentialLabel,
        sshHost,
        setSshHost,
        keyLabel,
        setKeyLabel,
        keys,
        keysError,
        credentials,
        selectedKey,
        toolsStatus,
        vmPoweredOn,
        guestOpsReady,
        vmwareReady,
        vmwareCanAttempt,
        sshReady,
        transportReady,
        transportMessage,
        transportMessageTone,
    };
}

export type VMTransportState = ReturnType<typeof useVMTransport>;
