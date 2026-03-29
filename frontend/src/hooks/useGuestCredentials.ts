import { useEffect, useState } from 'react';
import { config } from '../../wailsjs/go/models';
import { ListGuestCredentials } from '../../wailsjs/go/main/App';

export default function useGuestCredentials() {
    const [credentials, setCredentials] = useState<config.GuestCredentialMeta[]>([]);
    const [error, setError] = useState('');

    useEffect(() => {
        let cancelled = false;

        ListGuestCredentials()
            .then(list => {
                if (cancelled) return;
                setCredentials(list ?? []);
                setError('');
            })
            .catch((e: any) => {
                if (cancelled) return;
                setError(String(e));
            });

        return () => {
            cancelled = true;
        };
    }, []);

    return { credentials, error };
}
