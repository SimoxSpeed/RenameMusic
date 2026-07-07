export namespace main {
	
	export class ResultView {
	    oldName: string;
	    newName: string;
	    tagged: boolean;
	    skipped: boolean;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new ResultView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oldName = source["oldName"];
	        this.newName = source["newName"];
	        this.tagged = source["tagged"];
	        this.skipped = source["skipped"];
	        this.reason = source["reason"];
	    }
	}
	export class FileView {
	    name: string;
	    path: string;
	    preview: string;
	    mp3: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.preview = source["preview"];
	        this.mp3 = source["mp3"];
	    }
	}
	export class StateResponse {
	    folder: string;
	    files: FileView[];
	    logs: string[];
	    config: rules.Config;
	    destinationSameAsSource: boolean;
	    destinationFolder: string;
	    deleteOriginals: boolean;
	    watchEnabled: boolean;
	    watchActive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StateResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = source["folder"];
	        this.files = this.convertValues(source["files"], FileView);
	        this.logs = source["logs"];
	        this.config = this.convertValues(source["config"], rules.Config);
	        this.destinationSameAsSource = source["destinationSameAsSource"];
	        this.destinationFolder = source["destinationFolder"];
	        this.deleteOriginals = source["deleteOriginals"];
	        this.watchEnabled = source["watchEnabled"];
	        this.watchActive = source["watchActive"];
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
	export class ActionResponse {
	    ok: boolean;
	    message: string;
	    state: StateResponse;
	    results?: ResultView[];
	
	    static createFrom(source: any = {}) {
	        return new ActionResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.state = this.convertValues(source["state"], StateResponse);
	        this.results = this.convertValues(source["results"], ResultView);
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

export namespace rules {
	
	export class Replacement {
	    from: string;
	    to: string;
	
	    static createFrom(source: any = {}) {
	        return new Replacement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	    }
	}
	export class Config {
	    startFolder: string;
	    supportedExtensions: string[];
	    occurrenciesToRemove: string[];
	    occurrenciesToReplaceWithFt: string[];
	    replacements: Replacement[];
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startFolder = source["startFolder"];
	        this.supportedExtensions = source["supportedExtensions"];
	        this.occurrenciesToRemove = source["occurrenciesToRemove"];
	        this.occurrenciesToReplaceWithFt = source["occurrenciesToReplaceWithFt"];
	        this.replacements = this.convertValues(source["replacements"], Replacement);
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

