import { useState, useEffect, useCallback } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { SnapshotList, SnapshotCreate, SnapshotRevert, SnapshotDelete } from '../../../../wailsjs/go/manager/Manager';
import { EventsOn, EventsOff } from '../../../../wailsjs/runtime/runtime';

interface Props {
    vm: manager.VMInfo;
    onJobStarted: (id: string) => void;
}

export default function SnapshotsTab({ vm, onJobStarted }: Props) {
    const [snaps, setSnaps]       = useState<manager.SnapshotInfo[]>([]);
    const [loading, setLoading]   = useState(false);
    const [error, setError]       = useState('');
    const [newName, setNewName]   = useState('');
    const [newDesc, setNewDesc]   = useState('');
    const [memory, setMemory]     = useState(false);
    const [quiesce, setQuiesce]   = useState(false);
    const [creating, setCreating] = useState(false);
    const [createErr, setCreateErr] = useState('');

    const load = useCallback(async () => {
        setLoading(true);
        setError('');
        try {
            setSnaps(await SnapshotList(vm.ref) ?? []);
        } catch (e: any) {
            setError(String(e));
        } finally {
            setLoading(false);
        }
    }, [vm.ref]);

    useEffect(() => { load(); }, [load]);

    function trackAndReload(id: string) {
        onJobStarted(id);
        const unsub = EventsOn(`job:${id}`, (job: any) => {
            if (job.status === 'done' || job.status === 'failed' || job.status === 'cancelled') {
                EventsOff(`job:${id}`);
                unsub();
                load();
            }
        });
    }

    async function handleCreate() {
        if (!newName.trim()) return;
        setCreateErr('');
        setCreating(true);
        try {
            const id = await SnapshotCreate({ vmRef: vm.ref, name: newName.trim(), description: newDesc, memory, quiesce });
            setNewName('');
            setNewDesc('');
            trackAndReload(id);
        } catch (e: any) {
            setCreateErr(String(e));
        } finally {
            setCreating(false);
        }
    }

    function handleRevert(snapRef: string) {
        SnapshotRevert(snapRef).then(trackAndReload).catch((e: any) => setError(String(e)));
    }

    function handleDelete(snapRef: string) {
        SnapshotDelete(snapRef, false).then(trackAndReload).catch((e: any) => setError(String(e)));
    }

    return (
        <div className="tab-body">
            {/* Create form */}
            <div className="form-section">
                <div className="cred-row">
                    <div className="field field--inline">
                        <label>Snapshot name</label>
                        <input value={newName} onChange={e => setNewName(e.target.value)} placeholder="my-snapshot" />
                    </div>
                    <div className="field field--inline">
                        <label>Description</label>
                        <input value={newDesc} onChange={e => setNewDesc(e.target.value)} placeholder="optional" />
                    </div>
                </div>
                <div className="checkbox-group">
                    <label className="checkbox-row">
                        <input type="checkbox" checked={memory} onChange={e => setMemory(e.target.checked)} />
                        Include memory state
                    </label>
                    <label className="checkbox-row">
                        <input type="checkbox" checked={quiesce} onChange={e => setQuiesce(e.target.checked)} />
                        Quiesce guest filesystem (requires Tools)
                    </label>
                </div>
                {createErr && <p className="form-error">{createErr}</p>}
                <button className="btn-primary" onClick={handleCreate} disabled={creating || !newName.trim()}>
                    {creating ? 'Creating…' : 'Take Snapshot'}
                </button>
            </div>

            {/* Snapshot list */}
            <div className="snapshot-list-header">
                <span className="transfer-title">Snapshots</span>
                <button className="icon-btn" onClick={load} disabled={loading} title="Refresh">
                    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="23 4 23 10 17 10"/>
                        <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
                    </svg>
                </button>
            </div>

            {error && <p className="form-error">{error}</p>}
            {loading && <p className="vm-browser-empty">Loading…</p>}
            {!loading && snaps.length === 0 && !error && (
                <p className="vm-browser-empty">No snapshots.</p>
            )}

            {snaps.map(snap => (
                <div key={snap.ref} className={`snapshot-row ${snap.isCurrent ? 'snapshot-row--current' : ''}`}
                     style={{ paddingLeft: `${0.75 + snap.depth * 1.25}rem` }}>
                    <div className="snapshot-info">
                        <span className="snapshot-name">{snap.name}</span>
                        {snap.isCurrent && <span className="badge badge--green" style={{fontSize:'0.7rem'}}>current</span>}
                        {snap.description && <span className="snapshot-desc">{snap.description}</span>}
                        <span className="snapshot-date">{new Date(snap.createTime).toLocaleString()}</span>
                    </div>
                    <div className="snapshot-actions">
                        <button className="btn-secondary" onClick={() => handleRevert(snap.ref)}>Revert</button>
                        <button className="btn-secondary btn-danger" onClick={() => handleDelete(snap.ref)}>Delete</button>
                    </div>
                </div>
            ))}
        </div>
    );
}
