import { useState, useCallback, useEffect, useRef } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { JobCancel, JobDismiss, JobGet, JobList } from '../../wailsjs/go/jobs/Manager';

export interface LogEntry {
    progress: number;
    message: string;
    timestamp: string;
}

export interface Job {
    id: string;
    feature: string;
    label: string;
    status: 'running' | 'done' | 'cancelled' | 'failed';
    progress: number;
    message: string;
    error?: string;
    log: LogEntry[];
    startedAt: string;
    endedAt?: string;
    targetName?: string;
}

function normalizeJob(job: any): Job {
    return {
        ...job,
        status: job.status as Job['status'],
        log: (job.log ?? []) as LogEntry[],
        startedAt: String(job.startedAt ?? ''),
        endedAt: job.endedAt ? String(job.endedAt) : undefined,
    };
}

export function useJobs() {
    const [jobs, setJobs] = useState<Job[]>([]);
    const subscriptionsRef = useRef(new Map<string, () => void>());
    const jobTargetNamesRef = useRef(new Map<string, string>());

    const upsertJob = useCallback((job: Job) => {
        setJobs(prev => {
            const targetName = job.targetName ?? jobTargetNamesRef.current.get(job.id);
            const nextJob = targetName ? { ...job, targetName } : job;
            const exists = prev.some(j => j.id === job.id);
            if (exists) {
                return prev.map(j => j.id === job.id
                    ? { ...nextJob, targetName: nextJob.targetName ?? j.targetName }
                    : j
                );
            }
            return [...prev, nextJob];
        });
    }, []);

    const ensureTracked = useCallback((id: string) => {
        const existing = subscriptionsRef.current.get(id);
        if (existing) return existing;

        const runtimeUnsub = EventsOn(`job:${id}`, (job: Job) => {
            upsertJob(job);
        });
        const unsub = () => {
            runtimeUnsub();
            subscriptionsRef.current.delete(id);
        };
        subscriptionsRef.current.set(id, unsub);
        return unsub;
    }, [upsertJob]);

    useEffect(() => {
        let cancelled = false;

        JobList()
            .then(list => {
                if (cancelled) return;
                const existing = (list ?? []).map(normalizeJob);
                setJobs(existing);
                existing.forEach(job => ensureTracked(job.id));
            })
            .catch(() => { /* best-effort hydration */ });

        return () => {
            cancelled = true;
            subscriptionsRef.current.forEach(unsub => unsub());
            subscriptionsRef.current.clear();
        };
    }, [ensureTracked]);

    const trackJob = useCallback((id: string, targetName?: string) => {
        if (targetName) jobTargetNamesRef.current.set(id, targetName);
        const unsub = ensureTracked(id);
        JobGet(id)
            .then(job => {
                if (job) {
                    const normalized = normalizeJob(job);
                    upsertJob(targetName ? { ...normalized, targetName } : normalized);
                }
            })
            .catch(() => { /* best-effort hydration */ });
        return unsub;
    }, [ensureTracked, upsertJob]);

    const dismiss = useCallback((id: string) => {
        const unsub = subscriptionsRef.current.get(id);
        if (unsub) unsub();
        jobTargetNamesRef.current.delete(id);
        setJobs(prev => prev.filter(j => j.id !== id));
        JobDismiss(id);
    }, []);

    const cancel = useCallback((id: string) => {
        void JobCancel(id);
    }, []);

    return { jobs, trackJob, dismiss, cancel };
}
