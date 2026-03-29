export namespace config {
	
	export class ConnectRequest {
	    backendType: string;
	    url: string;
	    username: string;
	    password?: string;
	    insecure: boolean;
	    vmrunPath: string;
	    vmDir: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backendType = source["backendType"];
	        this.url = source["url"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.insecure = source["insecure"];
	        this.vmrunPath = source["vmrunPath"];
	        this.vmDir = source["vmDir"];
	    }
	}
	export class ConnectionInfo {
	    backendType: string;
	    displayName: string;
	    guestOps: boolean;
	    inventory: boolean;
	    toolsInstall: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backendType = source["backendType"];
	        this.displayName = source["displayName"];
	        this.guestOps = source["guestOps"];
	        this.inventory = source["inventory"];
	        this.toolsInstall = source["toolsInstall"];
	    }
	}
	export class GuestCredential {
	    label: string;
	    username: string;
	    password: string;

	    static createFrom(source: any = {}) {
	        return new GuestCredential(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.username = source["username"];
	        this.password = source["password"];
	    }
	}
	export class GuestCredentialMeta {
	    label: string;
	    username: string;

	    static createFrom(source: any = {}) {
	        return new GuestCredentialMeta(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.username = source["username"];
	    }
	}
	export class KeyMeta {
	    label: string;
	    algorithm: string;
	    defaultUser: string;
	    publicKey: string;
	
	    static createFrom(source: any = {}) {
	        return new KeyMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.algorithm = source["algorithm"];
	        this.defaultUser = source["defaultUser"];
	        this.publicKey = source["publicKey"];
	    }
	}

}

export namespace jobs {
	
	export class LogEntry {
	    progress: number;
	    message: string;
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.progress = source["progress"];
	        this.message = source["message"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Job {
	    id: string;
	    feature: string;
	    label: string;
	    status: string;
	    progress: number;
	    message: string;
	    error?: string;
	    log: LogEntry[];
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    endedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.feature = source["feature"];
	        this.label = source["label"];
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.message = source["message"];
	        this.error = source["error"];
	        this.log = this.convertValues(source["log"], LogEntry);
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.endedAt = this.convertValues(source["endedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace manager {
	
	export class CreateSnapshotRequest {
	    vmRef: string;
	    name: string;
	    description: string;
	    memory: boolean;
	    quiesce: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CreateSnapshotRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vmRef = source["vmRef"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.memory = source["memory"];
	        this.quiesce = source["quiesce"];
	    }
	}
	export class DatastoreInfo {
	    ref: string;
	    name: string;
	    type: string;
	    capacityGB: number;
	    freeGB: number;
	    accessible: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DatastoreInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = source["ref"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.capacityGB = source["capacityGB"];
	        this.freeGB = source["freeGB"];
	        this.accessible = source["accessible"];
	    }
	}
	export class DeploySSHKeyRequest {
	    label: string;
	    host: string;
	    port: number;
	    username: string;
	    password: string;
	    guestOS: string;
	
	    static createFrom(source: any = {}) {
	        return new DeploySSHKeyRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.guestOS = source["guestOS"];
	    }
	}
	export class DownloadRequest {
	    vmRef: string;
	    credentialLabel: string;
	    username: string;
	    password: string;
	    guestPath: string;
	    localPath: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vmRef = source["vmRef"];
	        this.credentialLabel = source["credentialLabel"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.guestPath = source["guestPath"];
	        this.localPath = source["localPath"];
	    }
	}
	export class HostInfo {
	    ref: string;
	    name: string;
	    connectionState: string;
	    powerState: string;
	    totalCPUMHz: number;
	    usedCPUMHz: number;
	    totalMemoryMB: number;
	    usedMemoryMB: number;
	    vmCount: number;
	
	    static createFrom(source: any = {}) {
	        return new HostInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = source["ref"];
	        this.name = source["name"];
	        this.connectionState = source["connectionState"];
	        this.powerState = source["powerState"];
	        this.totalCPUMHz = source["totalCPUMHz"];
	        this.usedCPUMHz = source["usedCPUMHz"];
	        this.totalMemoryMB = source["totalMemoryMB"];
	        this.usedMemoryMB = source["usedMemoryMB"];
	        this.vmCount = source["vmCount"];
	    }
	}
	export class InstallRequest {
	    vmRef: string;
	    credentialLabel: string;
	    username: string;
	    password: string;
	    localPath: string;
	    guestOS: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vmRef = source["vmRef"];
	        this.credentialLabel = source["credentialLabel"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.localPath = source["localPath"];
	        this.guestOS = source["guestOS"];
	        this.command = source["command"];
	    }
	}
	export class PortGroupInfo {
	    name: string;
	    vlan: string;
	    vmCount: number;
	    hosts: string[];
	
	    static createFrom(source: any = {}) {
	        return new PortGroupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.vlan = source["vlan"];
	        this.vmCount = source["vmCount"];
	        this.hosts = source["hosts"];
	    }
	}
	export class SwitchInfo {
	    name: string;
	    type: string;
	    mtu: number;
	    uplinks: string[];
	    hosts: string[];
	    portGroups: PortGroupInfo[];
	
	    static createFrom(source: any = {}) {
	        return new SwitchInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.mtu = source["mtu"];
	        this.uplinks = source["uplinks"];
	        this.hosts = source["hosts"];
	        this.portGroups = this.convertValues(source["portGroups"], PortGroupInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NetworkSummary {
	    switches: SwitchInfo[];
	
	    static createFrom(source: any = {}) {
	        return new NetworkSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.switches = this.convertValues(source["switches"], SwitchInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class RunRequest {
	    vmRef: string;
	    credentialLabel: string;
	    username: string;
	    password: string;
	    command: string;
	    guestOS: string;
	
	    static createFrom(source: any = {}) {
	        return new RunRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vmRef = source["vmRef"];
	        this.credentialLabel = source["credentialLabel"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.command = source["command"];
	        this.guestOS = source["guestOS"];
	    }
	}
	export class SSHInstallRequest {
	    host: string;
	    keyLabel: string;
	    localPath: string;
	    guestOS: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new SSHInstallRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.keyLabel = source["keyLabel"];
	        this.localPath = source["localPath"];
	        this.guestOS = source["guestOS"];
	        this.command = source["command"];
	    }
	}
	export class SSHRunRequest {
	    host: string;
	    keyLabel: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new SSHRunRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.keyLabel = source["keyLabel"];
	        this.command = source["command"];
	    }
	}
	export class SSHTransferRequest {
	    host: string;
	    keyLabel: string;
	    localPath: string;
	    guestPath: string;
	
	    static createFrom(source: any = {}) {
	        return new SSHTransferRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.keyLabel = source["keyLabel"];
	        this.localPath = source["localPath"];
	        this.guestPath = source["guestPath"];
	    }
	}
	export class SnapshotInfo {
	    ref: string;
	    name: string;
	    description: string;
	    // Go type: time
	    createTime: any;
	    isCurrent: boolean;
	    depth: number;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = source["ref"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.createTime = this.convertValues(source["createTime"], null);
	        this.isCurrent = source["isCurrent"];
	        this.depth = source["depth"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class UploadRequest {
	    vmRef: string;
	    credentialLabel: string;
	    username: string;
	    password: string;
	    localPath: string;
	    guestPath: string;
	    guestOS: string;
	
	    static createFrom(source: any = {}) {
	        return new UploadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vmRef = source["vmRef"];
	        this.credentialLabel = source["credentialLabel"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.localPath = source["localPath"];
	        this.guestPath = source["guestPath"];
	        this.guestOS = source["guestOS"];
	    }
	}
	export class VMInfo {
	    ref: string;
	    name: string;
	    pathSegments: string[];
	    displayPath: string;
	    powerState: string;
	    toolsStatus: string;
	    guestOS: string;
	    ipAddress: string;
	    numCPU: number;
	    memoryMB: number;
	
	    static createFrom(source: any = {}) {
	        return new VMInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = source["ref"];
	        this.name = source["name"];
	        this.pathSegments = source["pathSegments"];
	        this.displayPath = source["displayPath"];
	        this.powerState = source["powerState"];
	        this.toolsStatus = source["toolsStatus"];
	        this.guestOS = source["guestOS"];
	        this.ipAddress = source["ipAddress"];
	        this.numCPU = source["numCPU"];
	        this.memoryMB = source["memoryMB"];
	    }
	}

}
