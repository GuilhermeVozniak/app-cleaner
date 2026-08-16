export namespace backup {
	
	export class Item {
	    path: string;
	    name: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	    }
	}
	export class Details {
	    items: Item[];
	    fromManifest: boolean;
	    truncated: number;
	
	    static createFrom(source: any = {}) {
	        return new Details(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Item);
	        this.fromManifest = source["fromManifest"];
	        this.truncated = source["truncated"];
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
	export class Info {
	    path: string;
	    // Go type: time
	    date: any;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.date = this.convertValues(source["date"], null);
	        this.size = source["size"];
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
	
	export class RestoreResult {
	    restored: number;
	    failed: number;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new RestoreResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.restored = source["restored"];
	        this.failed = source["failed"];
	        this.errors = source["errors"];
	    }
	}

}

export namespace config {
	
	export class ExtraPaths {
	    nodeModules: string[];
	    projects: string[];
	
	    static createFrom(source: any = {}) {
	        return new ExtraPaths(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodeModules = source["nodeModules"];
	        this.projects = source["projects"];
	    }
	}
	export class Config {
	    downloadsDaysOld: number;
	    largeFilesMinSize: number;
	    backupByDefault: boolean;
	    backupRetentionDays: number;
	    concurrency: number;
	    showRisky: boolean;
	    keepLanguages: string[];
	    extraPaths: ExtraPaths;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadsDaysOld = source["downloadsDaysOld"];
	        this.largeFilesMinSize = source["largeFilesMinSize"];
	        this.backupByDefault = source["backupByDefault"];
	        this.backupRetentionDays = source["backupRetentionDays"];
	        this.concurrency = source["concurrency"];
	        this.showRisky = source["showRisky"];
	        this.keepLanguages = source["keepLanguages"];
	        this.extraPaths = this.convertValues(source["extraPaths"], ExtraPaths);
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

export namespace core {
	
	export class Category {
	    id: string;
	    name: string;
	    group: string;
	    description: string;
	    safetyLevel: string;
	    safetyNote?: string;
	    supportsFileSelection?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.description = source["description"];
	        this.safetyLevel = source["safetyLevel"];
	        this.safetyNote = source["safetyNote"];
	        this.supportsFileSelection = source["supportsFileSelection"];
	    }
	}
	export class CleanableItem {
	    path: string;
	    size: number;
	    name: string;
	    isDirectory: boolean;
	    // Go type: time
	    modifiedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new CleanableItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.size = source["size"];
	        this.name = source["name"];
	        this.isDirectory = source["isDirectory"];
	        this.modifiedAt = this.convertValues(source["modifiedAt"], null);
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
	export class ScanResult {
	    category: Category;
	    items: CleanableItem[];
	    totalSize: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category = this.convertValues(source["category"], Category);
	        this.items = this.convertValues(source["items"], CleanableItem);
	        this.totalSize = source["totalSize"];
	        this.error = source["error"];
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

export namespace grouping {
	
	export class DisplayRow {
	    type: string;
	    directoryKey: string;
	    displayName: string;
	    path?: string;
	    size?: number;
	    name?: string;
	    hiddenCount?: number;
	    totalFilesInDir: number;
	    selectable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DisplayRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.directoryKey = source["directoryKey"];
	        this.displayName = source["displayName"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.name = source["name"];
	        this.hiddenCount = source["hiddenCount"];
	        this.totalFilesInDir = source["totalFilesInDir"];
	        this.selectable = source["selectable"];
	    }
	}

}

export namespace loginitems {
	
	export class Item {
	    label: string;
	    path: string;
	    program: string;
	    kind: string;
	    runAtLoad: boolean;
	    programMissing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.path = source["path"];
	        this.program = source["program"];
	        this.kind = source["kind"];
	        this.runAtLoad = source["runAtLoad"];
	        this.programMissing = source["programMissing"];
	    }
	}

}

export namespace main {
	
	export class ActivityStats {
	    totalCleanedBytes: number;
	    totalCleanedItems: number;
	    cleanRuns: number;
	    scanRuns: number;
	    appsUninstalled: number;
	    lastCleanAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ActivityStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalCleanedBytes = source["totalCleanedBytes"];
	        this.totalCleanedItems = source["totalCleanedItems"];
	        this.cleanRuns = source["cleanRuns"];
	        this.scanRuns = source["scanRuns"];
	        this.appsUninstalled = source["appsUninstalled"];
	        this.lastCleanAt = source["lastCleanAt"];
	    }
	}
	export class CleanOptions {
	    dryRun: boolean;
	    backup: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CleanOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dryRun = source["dryRun"];
	        this.backup = source["backup"];
	    }
	}
	export class DiskUsage {
	    total: number;
	    free: number;
	    used: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.free = source["free"];
	        this.used = source["used"];
	    }
	}
	export class UpdateInfo {
	    current: string;
	    latest: string;
	    available: boolean;
	    url: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.available = source["available"];
	        this.url = source["url"];
	        this.error = source["error"];
	    }
	}

}

export namespace maintenance {
	
	export class Result {
	    success: boolean;
	    message: string;
	    error?: string;
	    requiresAdmin: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.error = source["error"];
	        this.requiresAdmin = source["requiresAdmin"];
	    }
	}

}

export namespace spacelens {
	
	export class Node {
	    name: string;
	    path: string;
	    size: number;
	    isDir: boolean;
	    children?: Node[];
	    truncated?: number;
	
	    static createFrom(source: any = {}) {
	        return new Node(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.isDir = source["isDir"];
	        this.children = this.convertValues(source["children"], Node);
	        this.truncated = source["truncated"];
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

export namespace uninstall {
	
	export class RelatedPath {
	    path: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new RelatedPath(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.size = source["size"];
	    }
	}
	export class AppInfo {
	    name: string;
	    path: string;
	    bundleId: string;
	    appSize: number;
	    relatedPaths: RelatedPath[];
	    totalSize: number;
	    running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.bundleId = source["bundleId"];
	        this.appSize = source["appSize"];
	        this.relatedPaths = this.convertValues(source["relatedPaths"], RelatedPath);
	        this.totalSize = source["totalSize"];
	        this.running = source["running"];
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

