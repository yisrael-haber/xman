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

interface JobUpdate {
    id: string;
    feature: string;
    label: string;
    status: Job['status'];
    progress: number;
    message: string;
    error?: string;
    startedAt: string;
    endedAt?: string;
    log?: LogEntry[];
    logLength?: number;
    lastLog?: LogEntry;
    targetName?: string;
}

function normalizeLogEntry(entry: any): LogEntry {
    return {
        progress: Number(entry?.progress ?? 0),
        message: String(entry?.message ?? ''),
        timestamp: String(entry?.timestamp ?? ''),
    };
}

function normalizeJob(job: any): JobUpdate {
    const log = Array.isArray(job?.log)
        ? job.log.map(normalizeLogEntry)
        : undefined;
    const lastLog = job?.lastLog
        ? normalizeLogEntry(job.lastLog)
        : log && log.length > 0
            ? log[log.length - 1]
            : undefined;

    return {
        id: String(job?.id ?? ''),
        feature: String(job?.feature ?? ''),
        label: String(job?.label ?? ''),
        status: job?.status as Job['status'],
        progress: Number(job?.progress ?? 0),
        message: String(job?.message ?? ''),
        error: job?.error ? String(job.error) : undefined,
        log,
        logLength: typeof job?.logLength === 'number' ? job.logLength : log?.length,
        lastLog,
        startedAt: String(job.startedAt ?? ''),
        endedAt: job.endedAt ? String(job.endedAt) : undefined,
        targetName: job?.targetName ? String(job.targetName) : undefined,
    };
}

function toJob(update: JobUpdate, existing?: Job, fallbackTargetName?: string): Job {
    const nextLog = update.log
        ? update.log
        : update.lastLog && (update.logLength ?? 0) > (existing?.log.length ?? 0)
            ? [...(existing?.log ?? []), update.lastLog]
            : (existing?.log ?? []);

    return {
        id: update.id,
        feature: update.feature,
        label: update.label,
        status: update.status,
        progress: update.progress,
        message: update.message,
        error: update.error,
        log: nextLog,
        startedAt: update.startedAt,
        endedAt: update.endedAt,
        targetName: update.targetName ?? fallbackTargetName ?? existing?.targetName,
    };
}

export function useJobs() {
    const [jobs, setJobs] = useState<Job[]>([]);
    const subscriptionsRef = useRef(new Map<string, () => void>());
    const jobTargetNamesRef = useRef(new Map<string, string>());

    const upsertJob = useCallback((job: JobUpdate) => {
        setJobs(prev => {
            const targetName = job.targetName ?? jobTargetNamesRef.current.get(job.id);
            const existing = prev.find(entry => entry.id === job.id);
            const nextJob = toJob(job, existing, targetName);

            if (existing) {
                return prev.map(entry => entry.id === job.id ? nextJob : entry);
            }
            return [...prev, nextJob];
        });
    }, []);

    const ensureTracked = useCallback((id: string) => {
        const existing = subscriptionsRef.current.get(id);
        if (existing) return existing;

        const runtimeUnsub = EventsOn(`job:${id}`, (job: JobUpdate) => {
            upsertJob(normalizeJob(job));
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
                const existing = (list ?? []).map(job => toJob(normalizeJob(job)));
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
