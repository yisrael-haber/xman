import { useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { manager } from '../../../../wailsjs/go/models';
import { formatGuestOpsStatus, formatPowerState, formatToolsStatus } from '../../../utils/vmStatus';

interface Props {
    vms: manager.VMInfo[];
    selected: manager.VMInfo | null;
    loading: boolean;
    error: string;
    width: number;
    onSelect: (vm: manager.VMInfo) => void;
    onRefresh: () => Promise<void>;
}

interface VMFolderNode {
    key: string;
    name: string;
    path: string[];
    folders: VMFolderNode[];
    vms: manager.VMInfo[];
}

interface MutableVMFolderNode extends VMFolderNode {
    childMap: Map<string, MutableVMFolderNode>;
}

const FOLDER_KEY_SEPARATOR = '\u001f';

function folderKey(path: string[]): string {
    return path.join(FOLDER_KEY_SEPARATOR);
}

function normalizeTreeLabel(value: string): string {
    return value
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '');
}

function hasRedundantTopLevelFolder(vm: manager.VMInfo, segments: string[]): boolean {
    if (segments.length !== 1) return false;

    const folderName = normalizeTreeLabel(segments[0] || '');
    const vmName = normalizeTreeLabel(vm.name || '');
    return folderName !== '' && folderName === vmName;
}

function buildTree(vms: manager.VMInfo[]): VMFolderNode[] {
    const root = new Map<string, MutableVMFolderNode>();
    const rootVMs: manager.VMInfo[] = [];

    for (const vm of vms) {
        const segments = vm.pathSegments ?? [];
        if (segments.length === 0 || hasRedundantTopLevelFolder(vm, segments)) {
            rootVMs.push(vm);
            continue;
        }

        let current = root;
        const currentPath: string[] = [];
        let currentFolder: MutableVMFolderNode | null = null;

        for (const segment of segments) {
            currentPath.push(segment);
            const key = folderKey(currentPath);
            let folder = current.get(key);
            if (!folder) {
                folder = {
                    key,
                    name: segment,
                    path: [...currentPath],
                    folders: [],
                    vms: [],
                    childMap: new Map(),
                };
                current.set(key, folder);
            }
            currentFolder = folder;
            current = folder.childMap;
        }

        currentFolder?.vms.push(vm);
    }

    const sortVMs = (items: manager.VMInfo[]) =>
        [...items].sort((left, right) =>
            left.name.localeCompare(right.name, undefined, { sensitivity: 'base', numeric: true }),
        );

    function finalizeFolders(folders: Map<string, MutableVMFolderNode>): VMFolderNode[] {
        return Array.from(folders.values())
            .map(folder => ({
                key: folder.key,
                name: folder.name,
                path: folder.path,
                folders: finalizeFolders(folder.childMap),
                vms: sortVMs(folder.vms),
            }))
            .sort((left, right) =>
                left.name.localeCompare(right.name, undefined, { sensitivity: 'base', numeric: true }),
            );
    }

    const tree = finalizeFolders(root);
    if (rootVMs.length > 0) {
        tree.push({
            key: '__root__',
            name: '',
            path: [],
            folders: [],
            vms: sortVMs(rootVMs),
        });
    }
    return tree;
}

function collectTopLevelFolderKeys(folders: VMFolderNode[]): string[] {
    return folders.filter(folder => folder.path.length > 0).map(folder => folder.key);
}

function selectedFolderKeys(selected: manager.VMInfo | null): string[] {
    if (!selected?.pathSegments?.length) return [];
    return selected.pathSegments.map((_, index) => folderKey(selected.pathSegments.slice(0, index + 1)));
}

function normalizeSearch(value: string): string {
    return value.trim().toLowerCase();
}

function vmSelfMatchesSearch(vm: manager.VMInfo, query: string): boolean {
    if (!query) return true;

    const haystack = [
        vm.name,
        vm.guestOS,
        vm.ipAddress,
        vm.ref,
    ]
        .filter(Boolean)
        .join('\n')
        .toLowerCase();

    return haystack.includes(query);
}

function vmMatchesSearch(vm: manager.VMInfo, query: string): boolean {
    if (vmSelfMatchesSearch(vm, query)) return true;

    const pathHaystack = [
        vm.displayPath,
        ...(vm.pathSegments ?? []),
    ]
        .filter(Boolean)
        .join('\n')
        .toLowerCase();

    return pathHaystack.includes(query);
}

function folderMatchesSearch(folder: VMFolderNode, query: string): boolean {
    if (!query || !folder.name) return false;

    const haystack = [folder.name, folder.path.join(' / ')]
        .filter(Boolean)
        .join('\n')
        .toLowerCase();

    return haystack.includes(query);
}

function collectSearchExpansionKeys(folders: VMFolderNode[], query: string): string[] {
    if (!query) return [];

    const keys = new Set<string>();

    function visit(folder: VMFolderNode): boolean {
        const folderDirectMatch = folderMatchesSearch(folder, query);
        const childNeedsOpen = folder.folders.some(child => visit(child));
        const directVMMatch = folder.vms.some(vm => vmSelfMatchesSearch(vm, query));
        const shouldOpen = folderDirectMatch || childNeedsOpen || directVMMatch;

        if (shouldOpen && folder.path.length > 0) {
            keys.add(folder.key);
        }
        return shouldOpen;
    }

    for (const folder of folders) {
        visit(folder);
    }

    return Array.from(keys);
}

function PowerDot({ state }: { state: string }) {
    const on = state === 'poweredOn';
    const suspended = state === 'suspended';
    return (
        <span
            className="power-dot"
            style={{ background: on ? '#4caf7d' : suspended ? '#e8a840' : '#445566' }}
            title={state}
        />
    );
}

function VMLeaf({ vm, selected, depth, onSelect }: {
    vm: manager.VMInfo;
    selected: manager.VMInfo | null;
    depth: number;
    onSelect: (vm: manager.VMInfo) => void;
}) {
    const toolsOk = vm.guestOpsReady || vm.toolsStatus === 'toolsOk' || vm.toolsStatus === 'toolsOld';
    const guestOpsStatus = formatGuestOpsStatus(vm.powerState, vm.guestOpsReady, vm.toolsStatus);

    return (
        <li
            className={`vm-item ${selected?.ref === vm.ref ? 'vm-item--active' : ''}`}
            onClick={() => onSelect(vm)}
            style={{ paddingLeft: `${0.8 + depth * 0.95}rem` }}
        >
            <PowerDot state={vm.powerState} />
            <div className="vm-item-body">
                <span
                    className={`vm-name ${!toolsOk ? 'vm-name--no-tools' : ''}`}
                    title={vm.displayPath ? `${vm.displayPath} / ${vm.name}` : vm.name}
                >
                    {vm.name}
                </span>
                <div className="vm-item-meta">
                    <span className={`vm-state-chip vm-state-chip--${vm.powerState}`}>
                        {formatPowerState(vm.powerState)}
                    </span>
                    <span
                        className={`vm-meta-text ${!vm.ipAddress && !toolsOk ? 'vm-meta-text--warn' : ''}`}
                        title={vm.ipAddress || guestOpsStatus || formatToolsStatus(vm.toolsStatus)}
                    >
                        {vm.ipAddress || guestOpsStatus || formatToolsStatus(vm.toolsStatus)}
                    </span>
                </div>
            </div>
        </li>
    );
}

function FolderBranch({
    folder,
    selected,
    expandedFolders,
    onToggle,
    onSelect,
    depth,
}: {
    folder: VMFolderNode;
    selected: manager.VMInfo | null;
    expandedFolders: Set<string>;
    onToggle: (key: string) => void;
    onSelect: (vm: manager.VMInfo) => void;
    depth: number;
}) {
    if (!folder.name) {
        return (
            <>
                {folder.vms.map(vm => (
                    <VMLeaf key={vm.ref} vm={vm} selected={selected} depth={depth} onSelect={onSelect} />
                ))}
            </>
        );
    }

    const expanded = expandedFolders.has(folder.key);

    return (
        <li className="vm-folder">
            <button
                type="button"
                className={`vm-folder-row ${expanded ? 'vm-folder-row--expanded' : ''}`}
                style={{ paddingLeft: `${0.8 + depth * 0.95}rem` }}
                onClick={() => onToggle(folder.key)}
                title={folder.path.join(' / ')}
            >
                <span className={`vm-folder-chevron ${expanded ? 'vm-folder-chevron--expanded' : ''}`}>
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="9 18 15 12 9 6" />
                    </svg>
                </span>
                <span className="vm-folder-icon">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M3 7.5A1.5 1.5 0 0 1 4.5 6H10l2 2h7.5A1.5 1.5 0 0 1 21 9.5v8A1.5 1.5 0 0 1 19.5 19h-15A1.5 1.5 0 0 1 3 17.5v-10Z" />
                    </svg>
                </span>
                <span className="vm-folder-name">{folder.name}</span>
            </button>

            {expanded && (
                <ul className="vm-tree">
                    {folder.folders.map(child => (
                        <FolderBranch
                            key={child.key}
                            folder={child}
                            selected={selected}
                            expandedFolders={expandedFolders}
                            onToggle={onToggle}
                            onSelect={onSelect}
                            depth={depth + 1}
                        />
                    ))}
                    {folder.vms.map(vm => (
                        <VMLeaf key={vm.ref} vm={vm} selected={selected} depth={depth + 1} onSelect={onSelect} />
                    ))}
                </ul>
            )}
        </li>
    );
}

export default function VMBrowser({ vms, selected, loading, error, width, onSelect, onRefresh }: Props) {
    const [search, setSearch] = useState('');
    const deferredSearch = useDeferredValue(search);
    const normalizedSearch = useMemo(() => normalizeSearch(deferredSearch), [deferredSearch]);
    const filteredVMs = useMemo(() => (
        normalizedSearch
            ? vms.filter(vm => vmMatchesSearch(vm, normalizedSearch))
            : vms
    ), [vms, normalizedSearch]);
    const tree = useMemo(() => buildTree(filteredVMs), [filteredVMs]);
    const topLevelKeys = useMemo(() => collectTopLevelFolderKeys(tree), [tree]);
    const topLevelKeysSignature = topLevelKeys.join(FOLDER_KEY_SEPARATOR);
    const selectedPathKey = folderKey(selected?.pathSegments ?? []);
    const searchExpansionKeys = useMemo(() => (
        normalizedSearch
            ? collectSearchExpansionKeys(tree, normalizedSearch)
            : []
    ), [tree, normalizedSearch]);
    const searchExpansionSignature = searchExpansionKeys.join(FOLDER_KEY_SEPARATOR);
    const [expandedFolders, setExpandedFolders] = useState<Set<string>>(new Set());
    const autoExpandedTopLevelsRef = useRef<Set<string>>(new Set());
    const lastSelectedPathRef = useRef('');

    useEffect(() => {
        if (normalizedSearch) {
            if (searchExpansionKeys.length === 0) return;

            setExpandedFolders(prev => {
                const next = new Set(prev);
                let changed = false;
                for (const key of searchExpansionKeys) {
                    if (!next.has(key)) {
                        next.add(key);
                        changed = true;
                    }
                }
                return changed ? next : prev;
            });
            return;
        }

        const newTopLevelKeys = topLevelKeys.filter(
            key => !autoExpandedTopLevelsRef.current.has(key),
        );
        for (const key of newTopLevelKeys) {
            autoExpandedTopLevelsRef.current.add(key);
        }

        const shouldOpenSelectedPath = selectedPathKey !== '' && selectedPathKey !== lastSelectedPathRef.current;
        lastSelectedPathRef.current = selectedPathKey;

        const keysToOpen = shouldOpenSelectedPath
            ? [...newTopLevelKeys, ...selectedFolderKeys(selected)]
            : newTopLevelKeys;

        if (keysToOpen.length === 0) return;

        setExpandedFolders(prev => {
            const next = new Set(prev);
            let changed = false;
            for (const key of keysToOpen) {
                if (!next.has(key)) {
                    next.add(key);
                    changed = true;
                }
            }
            return changed ? next : prev;
        });
    }, [normalizedSearch, searchExpansionSignature, selected, selectedPathKey, topLevelKeysSignature]);

    function toggleFolder(key: string) {
        setExpandedFolders(prev => {
            const next = new Set(prev);
            if (next.has(key)) {
                next.delete(key);
            } else {
                next.add(key);
            }
            return next;
        });
    }

    return (
        <div className="vm-browser" style={{ width: `${width}px` }}>
            <div className="vm-browser-header">
                <div className="vm-browser-toolbar">
                    <span className="vm-browser-title">Virtual Machines</span>
                    <button className="icon-btn" onClick={() => void onRefresh()} title="Refresh" disabled={loading}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <polyline points="23 4 23 10 17 10"/>
                            <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
                        </svg>
                    </button>
                </div>
                <div className="vm-browser-search">
                    <span className="vm-browser-search-icon" aria-hidden="true">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <circle cx="11" cy="11" r="7" />
                            <path d="m20 20-3.5-3.5" />
                        </svg>
                    </span>
                    <input
                        type="text"
                        value={search}
                        onChange={e => setSearch(e.target.value)}
                        placeholder="Search names or folders"
                        aria-label="Search virtual machines"
                    />
                    {search && (
                        <button
                            type="button"
                            className="vm-browser-search-clear"
                            onClick={() => setSearch('')}
                            title="Clear search"
                            aria-label="Clear search"
                        >
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <path d="M18 6 6 18" />
                                <path d="m6 6 12 12" />
                            </svg>
                        </button>
                    )}
                </div>
                {normalizedSearch && (
                    <p className="vm-browser-search-meta">
                        {filteredVMs.length} match{filteredVMs.length === 1 ? '' : 'es'}
                    </p>
                )}
            </div>

            {error && <p className="vm-browser-error">{error}</p>}

            {loading && vms.length === 0 && (
                <p className="vm-browser-empty">Loading...</p>
            )}

            {!loading && vms.length === 0 && !error && (
                <p className="vm-browser-empty">No VMs found.</p>
            )}

            {!loading && vms.length > 0 && filteredVMs.length === 0 && !error && (
                <p className="vm-browser-empty">No VMs match "{search.trim()}".</p>
            )}

            <ul className="vm-list vm-tree">
                {tree.map(folder => (
                    <FolderBranch
                        key={folder.key}
                        folder={folder}
                        selected={selected}
                        expandedFolders={expandedFolders}
                        onToggle={toggleFolder}
                        onSelect={onSelect}
                        depth={0}
                    />
                ))}
            </ul>
        </div>
    );
}
