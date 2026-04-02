import { useCallback, useEffect, useRef } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';

export interface TerminalJob {
    status: 'done' | 'failed' | 'cancelled' | 'running';
    error?: string;
    message?: string;
    log?: Array<{ progress?: number; message?: string }>;
    logLength?: number;
    lastLog?: { progress?: number; message?: string };
}

function isTerminalStatus(status: string): status is 'done' | 'failed' | 'cancelled' {
    return status === 'done' || status === 'failed' || status === 'cancelled';
}

export default function useTerminalJob() {
    const subscriptionsRef = useRef(new Map<string, () => void>());

    useEffect(() => () => {
        subscriptionsRef.current.forEach(unsub => unsub());
        subscriptionsRef.current.clear();
    }, []);

    return useCallback((id: string, onTerminal: (job: TerminalJob) => void) => {
        const existing = subscriptionsRef.current.get(id);
        if (existing) {
            return existing;
        }

        const runtimeUnsub = EventsOn(`job:${id}`, (job: TerminalJob) => {
            if (!isTerminalStatus(job.status)) {
                return;
            }

            const current = subscriptionsRef.current.get(id);
            if (current) {
                current();
            }
            onTerminal({
                ...job,
                log: Array.isArray(job.log) ? job.log : undefined,
                logLength: typeof job.logLength === 'number' ? job.logLength : undefined,
                lastLog: job.lastLog,
            });
        });

        const unsub = () => {
            runtimeUnsub();
            subscriptionsRef.current.delete(id);
        };

        subscriptionsRef.current.set(id, unsub);
        return unsub;
    }, []);
}
