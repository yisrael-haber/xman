export namespace config {
	
	export class ConnectionSettings {
	    url: string;
	    username: string;
	    insecure: boolean;
	    password?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.username = source["username"];
	        this.insecure = source["insecure"];
	        this.password = source["password"];
	    }
	}

}

export namespace filetransfer {
	
	export class DownloadRequest {
	    vmRef: string;
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
	        this.username = source["username"];
	        this.password = source["password"];
	        this.guestPath = source["guestPath"];
	        this.localPath = source["localPath"];
	    }
	}
	export class UploadRequest {
	    vmRef: string;
	    username: string;
	    password: string;
	    localPath: string;
	    guestPath: string;
	
	    static createFrom(source: any = {}) {
	        return new UploadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vmRef = source["vmRef"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.localPath = source["localPath"];
	        this.guestPath = source["guestPath"];
	    }
	}

}

export namespace guestexec {
	
	export class RunRequest {
	    vmRef: string;
	    username: string;
	    password: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new RunRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vmRef = source["vmRef"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.command = source["command"];
	    }
	}

}

export namespace inventory {
	
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

}

export namespace jobs {
	
	export class Job {
	    id: string;
	    feature: string;
	    label: string;
	    status: string;
	    progress: number;
	    message: string;
	    error?: string;
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

export namespace packetcapture {
	
	export class CaptureRequest {
	    vmRef: string;
	    username: string;
	    password: string;
	    interface: string;
	    duration: number;
	    localPath: string;
	
	    static createFrom(source: any = {}) {
	        return new CaptureRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vmRef = source["vmRef"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.interface = source["interface"];
	        this.duration = source["duration"];
	        this.localPath = source["localPath"];
	    }
	}

}

export namespace snapshots {
	
	export class CreateRequest {
	    vmRef: string;
	    name: string;
	    description: string;
	    memory: boolean;
	    quiesce: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CreateRequest(source);
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

}

export namespace vminfo {
	
	export class VMInfo {
	    ref: string;
	    name: string;
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
	        this.powerState = source["powerState"];
	        this.toolsStatus = source["toolsStatus"];
	        this.guestOS = source["guestOS"];
	        this.ipAddress = source["ipAddress"];
	        this.numCPU = source["numCPU"];
	        this.memoryMB = source["memoryMB"];
	    }
	}

}

