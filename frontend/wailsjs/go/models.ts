export namespace core {

	export class CPUInfo {
	    model: string;
	    cores: number;
	    logicalCpus: number;

	    static createFrom(source: any = {}) {
	        return new CPUInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.cores = source["cores"];
	        this.logicalCpus = source["logicalCpus"];
	    }
	}
	export class CUDAInfo {
	    available: boolean;
	    driverVersion: string;
	    toolkitVersion: string;

	    static createFrom(source: any = {}) {
	        return new CUDAInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.driverVersion = source["driverVersion"];
	        this.toolkitVersion = source["toolkitVersion"];
	    }
	}
	export class DlTask {
	    id: string;
	    modelId: string;
	    fileName: string;
	    destDir: string;
	    source: string;
	    status: string;
	    progress: number;
	    total: number;
	    downloaded: number;
	    sizeHuman: string;
	    speed: number;
	    error: string;

	    static createFrom(source: any = {}) {
	        return new DlTask(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.modelId = source["modelId"];
	        this.fileName = source["fileName"];
	        this.destDir = source["destDir"];
	        this.source = source["source"];
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.total = source["total"];
	        this.downloaded = source["downloaded"];
	        this.sizeHuman = source["sizeHuman"];
	        this.speed = source["speed"];
	        this.error = source["error"];
	    }
	}
	export class DownloadState {
	    status: string;
	    paused: boolean;
	    progress: number;
	    total: number;
	    downloaded: number;
	    fileName: string;
	    version: string;
	    error: string;

	    static createFrom(source: any = {}) {
	        return new DownloadState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.paused = source["paused"];
	        this.progress = source["progress"];
	        this.total = source["total"];
	        this.downloaded = source["downloaded"];
	        this.fileName = source["fileName"];
	        this.version = source["version"];
	        this.error = source["error"];
	    }
	}
	export class GPUInfo {
	    name: string;
	    memoryMb: number;
	    driverVersion: string;
	    cudaCores: number;

	    static createFrom(source: any = {}) {
	        return new GPUInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.memoryMb = source["memoryMb"];
	        this.driverVersion = source["driverVersion"];
	        this.cudaCores = source["cudaCores"];
	    }
	}
	export class HFFile {
	    rfilename: string;
	    size: number;

	    static createFrom(source: any = {}) {
	        return new HFFile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rfilename = source["rfilename"];
	        this.size = source["size"];
	    }
	}
	export class HFFileOut {
	    filename: string;
	    size: number;

	    static createFrom(source: any = {}) {
	        return new HFFileOut(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.size = source["size"];
	    }
	}
	export class HFSearchResult {
	    id: string;
	    modelId: string;
	    author: string;
	    downloads: number;
	    likes: number;
	    pipelineTag: string;
	    tags: string[];
	    siblings: HFFile[];

	    static createFrom(source: any = {}) {
	        return new HFSearchResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.modelId = source["modelId"];
	        this.author = source["author"];
	        this.downloads = source["downloads"];
	        this.likes = source["likes"];
	        this.pipelineTag = source["pipelineTag"];
	        this.tags = source["tags"];
	        this.siblings = this.convertValues(source["siblings"], HFFile);
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
	export class LlamaCppInfo {
	    installed: boolean;
	    path: string;
	    version: string;

	    static createFrom(source: any = {}) {
	        return new LlamaCppInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.path = source["path"];
	        this.version = source["version"];
	    }
	}
	export class MemoryInfo {
	    totalGb: number;
	    freeGb: number;

	    static createFrom(source: any = {}) {
	        return new MemoryInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalGb = source["totalGb"];
	        this.freeGb = source["freeGb"];
	    }
	}
	export class ModelConfig {
	    threads: number;
	    gpuLayers: string;
	    ctxSize: number;
	    batchSize: number;
	    ubatchSize: number;
	    flashAttn: boolean;
	    cacheTypeK: string;
	    cacheTypeV: string;
	    loadMode: string;
	    cpuMoe: boolean;
	    nCpuMoe: number;
	    splitMode: string;
	    tensorSplit: string;
	    mainGpu: number;
	    ropeScaling: string;
	    ropeScale: number;
	    mmproj: string;
	    reasoning: boolean;
	    specType: string;
	    specDraftNMax: number;
	    mlock?: boolean;
	    noMmap?: boolean;

	    static createFrom(source: any = {}) {
	        return new ModelConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.threads = source["threads"];
	        this.gpuLayers = source["gpuLayers"];
	        this.ctxSize = source["ctxSize"];
	        this.batchSize = source["batchSize"];
	        this.ubatchSize = source["ubatchSize"];
	        this.flashAttn = source["flashAttn"];
	        this.cacheTypeK = source["cacheTypeK"];
	        this.cacheTypeV = source["cacheTypeV"];
	        this.loadMode = source["loadMode"];
	        this.cpuMoe = source["cpuMoe"];
	        this.nCpuMoe = source["nCpuMoe"];
	        this.splitMode = source["splitMode"];
	        this.tensorSplit = source["tensorSplit"];
	        this.mainGpu = source["mainGpu"];
	        this.ropeScaling = source["ropeScaling"];
	        this.ropeScale = source["ropeScale"];
	        this.mmproj = source["mmproj"];
	        this.reasoning = source["reasoning"];
	        this.specType = source["specType"];
	        this.specDraftNMax = source["specDraftNMax"];
	        this.mlock = source["mlock"];
	        this.noMmap = source["noMmap"];
	    }
	}
	export class ModelInfo {
	    author: string;
	    name: string;
	    path: string;
	    sizeBytes: number;
	    sizeHuman: string;
	    architecture: string;
	    quantization: string;
	    hasMmproj: boolean;

	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.author = source["author"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.sizeBytes = source["sizeBytes"];
	        this.sizeHuman = source["sizeHuman"];
	        this.architecture = source["architecture"];
	        this.quantization = source["quantization"];
	        this.hasMmproj = source["hasMmproj"];
	    }
	}
	export class MonitorGPU {
	    index: number;
	    name: string;
	    utilPercent: number;
	    memUsed: number;
	    memTotal: number;

	    static createFrom(source: any = {}) {
	        return new MonitorGPU(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.utilPercent = source["utilPercent"];
	        this.memUsed = source["memUsed"];
	        this.memTotal = source["memTotal"];
	    }
	}
	export class MonitorStatus {
	    cpuPercent: number;
	    memUsed: number;
	    memTotal: number;
	    gpus: MonitorGPU[];
	    serverRunning: boolean;
	    tps: number;
	    uptimeSeconds: number;

	    static createFrom(source: any = {}) {
	        return new MonitorStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuPercent = source["cpuPercent"];
	        this.memUsed = source["memUsed"];
	        this.memTotal = source["memTotal"];
	        this.gpus = this.convertValues(source["gpus"], MonitorGPU);
	        this.serverRunning = source["serverRunning"];
	        this.tps = source["tps"];
	        this.uptimeSeconds = source["uptimeSeconds"];
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
	export class ServerConfig {
	    host: string;
	    port: number;
	    maxModels: number;
	    cacheRam: number;

	    static createFrom(source: any = {}) {
	        return new ServerConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.maxModels = source["maxModels"];
	        this.cacheRam = source["cacheRam"];
	    }
	}
	export class SystemInfo {
	    os: string;
	    arch: string;
	    cpu: CPUInfo;
	    memory: MemoryInfo;
	    gpu: GPUInfo[];
	    cuda: CUDAInfo;
	    llamaCpp: LlamaCppInfo;

	    static createFrom(source: any = {}) {
	        return new SystemInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.cpu = this.convertValues(source["cpu"], CPUInfo);
	        this.memory = this.convertValues(source["memory"], MemoryInfo);
	        this.gpu = this.convertValues(source["gpu"], GPUInfo);
	        this.cuda = this.convertValues(source["cuda"], CUDAInfo);
	        this.llamaCpp = this.convertValues(source["llamaCpp"], LlamaCppInfo);
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
	export class UpdateCheckResult {
	    hasUpdate: boolean;
	    version: string;
	    notes: string;
	    published: string;

	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasUpdate = source["hasUpdate"];
	        this.version = source["version"];
	        this.notes = source["notes"];
	        this.published = source["published"];
	    }
	}
	export class UpdateDownloadState {
	    status: string;
	    progress: number;
	    total: number;
	    downloaded: number;
	    version: string;
	    filePath: string;
	    error: string;

	    static createFrom(source: any = {}) {
	        return new UpdateDownloadState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.total = source["total"];
	        this.downloaded = source["downloaded"];
	        this.version = source["version"];
	        this.filePath = source["filePath"];
	        this.error = source["error"];
	    }
	}

}

