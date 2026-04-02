import { useEffect, useRef, useState } from 'react';
import { manager } from '../../wailsjs/go/models';
import useSSHKeys from './useSSHKeys';
import useGuestCredentials from './useGuestCredentials';

export type VMTransportMode = 'vmware' | 'ssh';

export default function useVMTransport(vm: manager.VMInfo, initialMode: VMTransportMode = 'vmware') {
    const [mode, setMode] = useState<VMTransportMode>(initialMode);
    const [credentialLabel, setCredentialLabel] = useState('');
    const [sshHost, setSshHostState] = useState(vm.ipAddress || '');
    const [keyLabel, setKeyLabel] = useState('');
    const { keys, error: keysError } = useSSHKeys();
    const { credentials, error: credentialsError } = useGuestCredentials();
    const previousVMRef = useRef(vm.ref);
    const sshHostTouchedRef = useRef(false);

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
    const sshUser = selectedKey?.defaultUser?.trim() || '';

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
        credentialsError,
        selectedKey,
        sshUser,
        toolsStatus: vm.toolsStatus,
        vmPoweredOn: vm.powerState === 'poweredOn',
        guestOpsReady: !!vm.guestOpsReady,
    };
}

export type VMTransportState = ReturnType<typeof useVMTransport>;
