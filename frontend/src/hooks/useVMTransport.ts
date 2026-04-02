import { useEffect, useState } from 'react';
import { manager } from '../../wailsjs/go/models';
import useSSHKeys from './useSSHKeys';
import useGuestCredentials from './useGuestCredentials';

export type VMTransportMode = 'vmware' | 'ssh';

export default function useVMTransport(vm: manager.VMInfo, initialMode: VMTransportMode = 'vmware') {
    const [mode, setMode] = useState<VMTransportMode>(initialMode);
    const [credentialLabel, setCredentialLabel] = useState('');
    const [sshHost, setSshHost] = useState(vm.ipAddress || '');
    const [keyLabel, setKeyLabel] = useState('');
    const { keys, error: keysError } = useSSHKeys();
    const { credentials, error: credentialsError } = useGuestCredentials();

    useEffect(() => {
        setSshHost(vm.ipAddress || '');
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
