import { useState } from 'react';
import { Job, LogEntry } from '../hooks/useJobs';

interface Props {
    jobs: Job[];
    onDismiss: (id: string) => void;
    onCancel: (id: string) => void;
}

function isCancellableJob(job: Job): boolean {
    return job.status === 'running' && (job.feature === 'guestexec' || job.feature === 'exec');
}

function formatTime(ts: string): string {
    const d = new Date(ts);
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function formatDuration(startedAt?: string, endedAt?: string): string | null {
    if (!startedAt || !endedAt) return null;

    const start = new Date(startedAt).getTime();
    const end = new Date(endedAt).getTime();
    if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return null;

    const totalSeconds = Math.max(0, Math.round((end - start) / 1000));
    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = totalSeconds % 60;

    if (hours > 0) return `${hours}h ${minutes}m ${seconds}s`;
    if (minutes > 0) return `${minutes}m ${seconds}s`;
    return `${seconds}s`;
}

function JobLogEntry({ entry }: { entry: LogEntry }) {
    return (
        <div className="job-log-entry">
            <span className="job-log-time">{formatTime(entry.timestamp)}</span>
            <span className="job-log-progress">{entry.progress}%</span>
            <span className="job-log-message">{entry.message}</span>
        </div>
    );
}

function JobItem({ job, onDismiss, onCancel }: { job: Job; onDismiss: (id: string) => void; onCancel: (id: string) => void }) {
    const [expanded, setExpanded] = useState(false);
    const hasLog = job.log && job.log.length > 0;
    const finished = job.status !== 'running';
    const duration = finished ? formatDuration(job.startedAt, job.endedAt) : null;

    return (
        <div className={`job-item job-item--${job.status}`}>
            <div className="job-item-header">
                <div className="job-info">
                    <div className="job-title-row">
                        <span className="job-label">{job.label}</span>
                        {job.targetName && <span className="job-target">{job.targetName}</span>}
                        {duration && <span className="job-duration">Took {duration}</span>}
                    </div>
                    {job.message && <span className="job-message">{job.message}</span>}
                    {job.error && <span className="job-error">{job.error}</span>}
                </div>
                <div className="job-actions">
                    {isCancellableJob(job) && (
                        <button className="job-cancel" onClick={() => onCancel(job.id)} title="Cancel">Cancel</button>
                    )}
                    {hasLog && (
                        <button
                            className={`job-expand ${expanded ? 'job-expand--open' : ''}`}
                            onClick={() => setExpanded(e => !e)}
                            title={expanded ? 'Hide log' : 'Show log'}
                        >
                            ▶
                        </button>
                    )}
                    {finished && (
                        <button className="job-dismiss" onClick={() => onDismiss(job.id)} title="Dismiss">×</button>
                    )}
                </div>
            </div>

            {!finished && (
                <div className="job-progress-track">
                    <div className="job-progress-fill" style={{ width: `${job.progress}%` }} />
                </div>
            )}

            {expanded && hasLog && (
                <div className="job-log">
                    {job.log.map((entry, i) => (
                        <JobLogEntry key={i} entry={entry} />
                    ))}
                </div>
            )}
        </div>
    );
}

export default function JobsBar({ jobs, onDismiss, onCancel }: Props) {
    if (jobs.length === 0) return null;

    return (
        <div className="jobs-bar">
            {jobs.map(job => (
                <JobItem key={job.id} job={job} onDismiss={onDismiss} onCancel={onCancel} />
            ))}
        </div>
    );
}
