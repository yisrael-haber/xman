import { Job } from '../hooks/useJobs';

interface Props {
    jobs: Job[];
    onDismiss: (id: string) => void;
}

export default function JobsBar({ jobs, onDismiss }: Props) {
    if (jobs.length === 0) return null;

    return (
        <div className="jobs-bar">
            {jobs.map(job => (
                <div key={job.id} className={`job-item job-item--${job.status}`}>
                    <div className="job-info">
                        <span className="job-label">{job.label}</span>
                        {job.message && <span className="job-message">{job.message}</span>}
                        {job.error && <span className="job-error">{job.error}</span>}
                    </div>
                    {job.status === 'running' && (
                        <div className="job-progress-track">
                            <div className="job-progress-fill" style={{ width: `${job.progress}%` }} />
                        </div>
                    )}
                    {job.status !== 'running' && (
                        <button className="job-dismiss" onClick={() => onDismiss(job.id)} title="Dismiss">×</button>
                    )}
                </div>
            ))}
        </div>
    );
}
