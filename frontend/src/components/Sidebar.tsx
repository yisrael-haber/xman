export type FeatureID = 'vms' | 'inventory' | 'networks';

interface NavItem {
    id: FeatureID;
    label: string;
    icon: JSX.Element;
}

interface Props {
    active: FeatureID;
    onChange: (id: FeatureID) => void;
    showInventory: boolean;
}

const VMsIcon = () => (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="2" y="3" width="20" height="14" rx="2"/>
        <line x1="8" y1="21" x2="16" y2="21"/>
        <line x1="12" y1="17" x2="12" y2="21"/>
    </svg>
);

const InventoryIcon = () => (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <ellipse cx="12" cy="5" rx="9" ry="3"/>
        <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
        <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
    </svg>
);

const NetworksIcon = () => (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="2" y="2" width="6" height="6" rx="1"/>
        <rect x="16" y="2" width="6" height="6" rx="1"/>
        <rect x="9" y="16" width="6" height="6" rx="1"/>
        <path d="M5 8v4a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8"/>
        <line x1="12" y1="14" x2="12" y2="16"/>
    </svg>
);

const NAV_ITEMS: NavItem[] = [
    { id: 'vms',       label: 'Virtual Machines',   icon: <VMsIcon /> },
    { id: 'networks',  label: 'Networks',            icon: <NetworksIcon /> },
    { id: 'inventory', label: 'Hosts & Datastores', icon: <InventoryIcon /> },
];

export default function Sidebar({ active, onChange, showInventory }: Props) {
    const items = NAV_ITEMS.filter(item => item.id !== 'inventory' || showInventory);
    return (
        <nav className="sidebar">
            {items.map(item => (
                <button
                    key={item.id}
                    className={`nav-item ${active === item.id ? 'nav-item--active' : ''}`}
                    onClick={() => onChange(item.id)}
                    title={item.label}
                >
                    {item.icon}
                </button>
            ))}
        </nav>
    );
}
