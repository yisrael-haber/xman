import type { TerminalJobUpdate } from '../hooks/useJobs';

export function extractTerminalOutput(job: TerminalJobUpdate): string {
    const log = Array.isArray(job?.log) ? job.log : [];
    for (let i = log.length - 1; i >= 0; i -= 1) {
        const entry = log[i];
        if (entry?.progress === 95 && typeof entry.message === 'string' && entry.message.trim()) {
            return entry.message;
        }
    }

    if (job?.lastLog?.progress === 95 && typeof job.lastLog.message === 'string' && job.lastLog.message.trim()) {
        return job.lastLog.message;
    }

    if (job?.status === 'cancelled') {
        return 'Command cancelled.';
    }

    if (typeof job?.error === 'string' && job.error.trim()) {
        return job.error;
    }

    if (typeof job?.message === 'string' && job.message.trim()) {
        return job.message;
    }

    return '(no output)';
}
