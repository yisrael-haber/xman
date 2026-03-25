import { useState, useEffect } from 'react';
import { vminfo } from '../../../../wailsjs/go/models';
import { VMList } from '../../../../wailsjs/go/vminfo/Binding';
import VMBrowser from './VMBrowser';
import VMInfoTab from './VMInfoTab';
import FileTransferTab from './FileTransferTab';
import PacketCaptureTab from './PacketCaptureTab';
import SnapshotsTab from './SnapshotsTab';
import GuestExecTab from './GuestExecTab';

type TabID = 'info' | 'filetransfer' | 'capture' | 'snapshots' | 'exec';

const TABS: { id: TabID; label: string }[] = [
    { id: 'info',         label: 'VM Info'         },
    { id: 'snapshots',    label: 'Snapshots'        },
    { id: 'exec',         label: 'Run Command'      },
    { id: 'filetransfer', label: 'File Transfer'    },
    { id: 'capture',      label: 'Packet Capture'   },
];

interface Props {
    onJobStarted: (id: string) => void;
}

export default function VMPanel({ onJobStarted }: Props) {
    const [vms,      setVms]      = useState<vminfo.VMInfo[]>([]);
    const [selected, setSelected] = useState<vminfo.VMInfo | null>(null);
    const [loading,  setLoading]  = useState(false);
    const [error,    setError]    = useState('');
    const [activeTab, setActiveTab] = useState<TabID>('info');

    async function loadVMs() {
        setLoading(true);
        setError('');
        try {
            const list = await VMList();
            setVms(list ?? []);
            setSelected(prev =>
                prev ? (list.find(v => v.ref === prev.ref) ?? null) : null
            );
        } catch (e: any) {
            setError(String(e));
        } finally {
            setLoading(false);
        }
    }

    useEffect(() => { loadVMs(); }, []);

    return (
        <div className="vm-panel">
            <VMBrowser
                vms={vms}
                selected={selected}
                loading={loading}
                error={error}
                onSelect={setSelected}
                onRefresh={loadVMs}
            />

            <div className="vm-detail">
                {!selected ? (
                    <div className="vm-placeholder">Select a VM to get started.</div>
                ) : (
                    <>
                        <div className="tab-bar">
                            {TABS.map(tab => (
                                <button
                                    key={tab.id}
                                    className={`tab ${activeTab === tab.id ? 'tab--active' : ''}`}
                                    onClick={() => setActiveTab(tab.id)}
                                >
                                    {tab.label}
                                </button>
                            ))}
                        </div>

                        <div className="tab-content">
                            {activeTab === 'info' && (
                                <VMInfoTab vm={selected} onRefresh={loadVMs} />
                            )}
                            {activeTab === 'snapshots' && (
                                <SnapshotsTab vm={selected} onJobStarted={onJobStarted} />
                            )}
                            {activeTab === 'exec' && (
                                <GuestExecTab vm={selected} onJobStarted={onJobStarted} />
                            )}
                            {activeTab === 'filetransfer' && (
                                <FileTransferTab vm={selected} onJobStarted={onJobStarted} />
                            )}
                            {activeTab === 'capture' && (
                                <PacketCaptureTab vm={selected} onJobStarted={onJobStarted} />
                            )}
                        </div>
                    </>
                )}
            </div>
        </div>
    );
}
