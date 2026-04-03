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

export interface JobUpdate {
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

export interface TerminalJobUpdate extends JobUpdate {
    status: Exclude<Job['status'], 'running'>;
}

export type TrackJobHandler = (id: string, targetName?: string) => void;
export type WatchTerminalJobHandler = (id: string, onTerminal: (job: TerminalJobUpdate) => void) => () => void;

function isTerminalStatus(status: Job['status']): status is TerminalJobUpdate['status'] {
    return status === 'done' || status === 'failed' || status === 'cancelled';
}

function toTerminalJob(job: JobUpdate): TerminalJobUpdate | null {
    return isTerminalStatus(job.status)
        ? { ...job, status: job.status }
        : null;
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
    const terminalWatchersRef = useRef(new Map<string, Set<(job: TerminalJobUpdate) => void>>());

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

    const notifyTerminalWatchers = useCallback((job: JobUpdate) => {
        const terminalJob = toTerminalJob(job);
        if (!terminalJob) {
            return;
        }

        const watchers = terminalWatchersRef.current.get(terminalJob.id);
        if (!watchers || watchers.size === 0) {
            return;
        }

        terminalWatchersRef.current.delete(terminalJob.id);
        watchers.forEach(watcher => watcher(terminalJob));
    }, []);

    const ensureTracked = useCallback((id: string) => {
        const existing = subscriptionsRef.current.get(id);
        if (existing) return existing;

        const runtimeUnsub = EventsOn(`job:${id}`, (job: JobUpdate) => {
            const normalized = normalizeJob(job);
            upsertJob(normalized);
            notifyTerminalWatchers(normalized);
        });
        const unsub = () => {
            runtimeUnsub();
            subscriptionsRef.current.delete(id);
        };
        subscriptionsRef.current.set(id, unsub);
        return unsub;
    }, [notifyTerminalWatchers, upsertJob]);

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
            terminalWatchersRef.current.clear();
        };
    }, [ensureTracked]);

    const trackJob = useCallback<TrackJobHandler>((id, targetName) => {
        if (targetName) jobTargetNamesRef.current.set(id, targetName);
        const unsub = ensureTracked(id);
        JobGet(id)
            .then(job => {
                if (job) {
                    const normalized = normalizeJob(job);
                    const nextJob = targetName ? { ...normalized, targetName } : normalized;
                    upsertJob(nextJob);
                    notifyTerminalWatchers(nextJob);
                }
            })
            .catch(() => { /* best-effort hydration */ });
        return unsub;
    }, [ensureTracked, notifyTerminalWatchers, upsertJob]);

    const watchTerminalJob = useCallback<WatchTerminalJobHandler>((id, onTerminal) => {
        ensureTracked(id);

        let active = true;
        const watchers = terminalWatchersRef.current.get(id) ?? new Set<(job: TerminalJobUpdate) => void>();
        terminalWatchersRef.current.set(id, watchers);

        const remove = () => {
            if (!active) {
                return;
            }
            active = false;

            const currentWatchers = terminalWatchersRef.current.get(id);
            if (!currentWatchers) {
                return;
            }

            currentWatchers.delete(handleTerminal);
            if (currentWatchers.size === 0) {
                terminalWatchersRef.current.delete(id);
            }
        };

        const handleTerminal = (job: TerminalJobUpdate) => {
            if (!active) {
                return;
            }
            remove();
            onTerminal(job);
        };

        watchers.add(handleTerminal);

        JobGet(id)
            .then(job => {
                if (!active || !job) {
                    return;
                }

                const normalized = normalizeJob(job);
                const terminalJob = toTerminalJob(normalized);
                if (terminalJob) {
                    handleTerminal(terminalJob);
                }
            })
            .catch(() => { /* best-effort hydration */ });

        return remove;
    }, [ensureTracked]);

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

    return { jobs, trackJob, watchTerminalJob, dismiss, cancel };
}
