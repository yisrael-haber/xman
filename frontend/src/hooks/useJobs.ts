import { useState, useCallback } from 'react';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';

export interface Job {
    id: string;
    feature: string;
    label: string;
    status: 'running' | 'done' | 'cancelled' | 'failed';
    progress: number;
    message: string;
    error?: string;
}

export function useJobs() {
    const [jobs, setJobs] = useState<Job[]>([]);

    const trackJob = useCallback((id: string) => {
        const unsub = EventsOn(`job:${id}`, (job: Job) => {
            setJobs(prev => {
                const exists = prev.some(j => j.id === id);
                if (exists) return prev.map(j => j.id === id ? job : j);
                return [...prev, job];
            });
        });
        return unsub;
    }, []);

    const dismiss = useCallback((id: string) => {
        EventsOff(`job:${id}`);
        setJobs(prev => prev.filter(j => j.id !== id));
    }, []);

    const activeJobs = jobs.filter(j => j.status === 'running');
    const finishedJobs = jobs.filter(j => j.status !== 'running');

    return { jobs, activeJobs, finishedJobs, trackJob, dismiss };
}
