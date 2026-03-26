import { useState } from 'react';
import { Job, LogEntry } from '../hooks/useJobs';

interface Props {
    jobs: Job[];
    onDismiss: (id: string) => void;
}

function formatTime(ts: string): string {
    const d = new Date(ts);
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
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

function JobItem({ job, onDismiss }: { job: Job; onDismiss: (id: string) => void }) {
    const [expanded, setExpanded] = useState(false);
    const hasLog = job.log && job.log.length > 0;
    const finished = job.status !== 'running';

    return (
        <div className={`job-item job-item--${job.status}`}>
            <div className="job-item-header">
                <div className="job-info">
                    <span className="job-label">{job.label}</span>
                    {job.message && <span className="job-message">{job.message}</span>}
                    {job.error && <span className="job-error">{job.error}</span>}
                </div>
                <div className="job-actions">
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

export default function JobsBar({ jobs, onDismiss }: Props) {
    if (jobs.length === 0) return null;

    return (
        <div className="jobs-bar">
            {jobs.map(job => (
                <JobItem key={job.id} job={job} onDismiss={onDismiss} />
            ))}
        </div>
    );
}
