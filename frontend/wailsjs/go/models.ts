export namespace main {
	
	export class DownloadErrorView {
	    videoId: string;
	    title: string;
	    url: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadErrorView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.videoId = source["videoId"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.message = source["message"];
	    }
	}
	export class TagPromptView {
	    path: string;
	    originalBase: string;
	    ext: string;
	    title: string;
	    artist: string;
	
	    static createFrom(source: any = {}) {
	        return new TagPromptView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.originalBase = source["originalBase"];
	        this.ext = source["ext"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	    }
	}
	export class ResultView {
	    oldName: string;
	    newName: string;
	    tagged: boolean;
	    skipped: boolean;
	    failed: boolean;
	    canceled: boolean;
	    reason: string;
	    mp3: boolean;
	    title?: string;
	    artist?: string;
	
	    static createFrom(source: any = {}) {
	        return new ResultView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oldName = source["oldName"];
	        this.newName = source["newName"];
	        this.tagged = source["tagged"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.canceled = source["canceled"];
	        this.reason = source["reason"];
	        this.mp3 = source["mp3"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	    }
	}
	export class LogEntry {
	    time: string;
	    kind: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.kind = source["kind"];
	        this.message = source["message"];
	    }
	}
	export class FileView {
	    name: string;
	    path: string;
	    preview: string;
	    mp3: boolean;
	    title?: string;
	    artist?: string;
	    titlePreview?: string;
	    artistPreview?: string;
	
	    static createFrom(source: any = {}) {
	        return new FileView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.preview = source["preview"];
	        this.mp3 = source["mp3"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.titlePreview = source["titlePreview"];
	        this.artistPreview = source["artistPreview"];
	    }
	}
	export class StateResponse {
	    folder: string;
	    files: FileView[];
	    logs: LogEntry[];
	    config: rules.Config;
	    destinationSameAsSource: boolean;
	    destinationFolder: string;
	    deleteOriginals: boolean;
	    watchEnabled: boolean;
	    watchActive: boolean;
	    playlists: playlist.Playlist[];
	    ytDlpManaged: boolean;
	    ytDlpPath: string;
	    ytDlpEffectivePath: string;
	    ytDlpAvailable: boolean;
	    ytDlpVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new StateResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = source["folder"];
	        this.files = this.convertValues(source["files"], FileView);
	        this.logs = this.convertValues(source["logs"], LogEntry);
	        this.config = this.convertValues(source["config"], rules.Config);
	        this.destinationSameAsSource = source["destinationSameAsSource"];
	        this.destinationFolder = source["destinationFolder"];
	        this.deleteOriginals = source["deleteOriginals"];
	        this.watchEnabled = source["watchEnabled"];
	        this.watchActive = source["watchActive"];
	        this.playlists = this.convertValues(source["playlists"], playlist.Playlist);
	        this.ytDlpManaged = source["ytDlpManaged"];
	        this.ytDlpPath = source["ytDlpPath"];
	        this.ytDlpEffectivePath = source["ytDlpEffectivePath"];
	        this.ytDlpAvailable = source["ytDlpAvailable"];
	        this.ytDlpVersion = source["ytDlpVersion"];
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
	    prompts?: TagPromptView[];
	    downloadErrors?: DownloadErrorView[];
	
	    static createFrom(source: any = {}) {
	        return new ActionResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.state = this.convertValues(source["state"], StateResponse);
	        this.results = this.convertValues(source["results"], ResultView);
	        this.prompts = this.convertValues(source["prompts"], TagPromptView);
	        this.downloadErrors = this.convertValues(source["downloadErrors"], DownloadErrorView);
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

export namespace playlist {
	
	export class Playlist {
	    name: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new Playlist(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	    }
	}

}

export namespace rules {
	
	export class Replacement {
	    from: string;
	    to: string;
	    scope?: string;
	
	    static createFrom(source: any = {}) {
	        return new Replacement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.scope = source["scope"];
	    }
	}
	export class Config {
	    startFolder: string;
	    supportedExtensions: string[];
	    occurrenciesToRemove: string[];
	    occurrenciesToReplaceWithFt: string[];
	    replacements: Replacement[];
	    ftAlias: string;
	    artistExceptions: string[];
	
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
	        this.ftAlias = source["ftAlias"];
	        this.artistExceptions = source["artistExceptions"];
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

