import { useEffect, useState } from 'react';
import { config } from '../../wailsjs/go/models';
import { ListSSHKeys } from '../../wailsjs/go/main/App';

export default function useSSHKeys() {
    const [keys, setKeys] = useState<config.KeyMeta[]>([]);
    const [error, setError] = useState('');

    useEffect(() => {
        let cancelled = false;

        ListSSHKeys()
            .then(list => {
                if (cancelled) return;
                setKeys(list ?? []);
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

    return { keys, error };
}
