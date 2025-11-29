export namespace main {
	
	export class ServerStatus {
	    running: boolean;
	    port: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.port = source["port"];
	        this.error = source["error"];
	    }
	}

}

export namespace models {
	
	export class ResponseGroup {
	    id?: string;
	    name: string;
	    expanded?: boolean;
	    enabled?: boolean;
	    responses?: MethodResponse[];
	
	    static createFrom(source: any = {}) {
	        return new ResponseGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.expanded = source["expanded"];
	        this.enabled = source["enabled"];
	        this.responses = this.convertValues(source["responses"], MethodResponse);
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
	export class ResponseItem {
	    type: string;
	    response?: MethodResponse;
	    group?: ResponseGroup;
	
	    static createFrom(source: any = {}) {
	        return new ResponseItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.response = this.convertValues(source["response"], MethodResponse);
	        this.group = this.convertValues(source["group"], ResponseGroup);
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
	export class RequestValidation {
	    mode?: string;
	    pattern?: string;
	    match_type?: string;
	    script?: string;
	
	    static createFrom(source: any = {}) {
	        return new RequestValidation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.pattern = source["pattern"];
	        this.match_type = source["match_type"];
	        this.script = source["script"];
	    }
	}
	export class MethodResponse {
	    id?: string;
	    enabled?: boolean;
	    path_pattern: string;
	    methods: string[];
	    status_code: number;
	    status_text?: string;
	    headers?: Record<string, string>;
	    body?: string;
	    response_delay?: number;
	    response_mode?: string;
	    script_body?: string;
	    request_validation?: RequestValidation;
	
	    static createFrom(source: any = {}) {
	        return new MethodResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.enabled = source["enabled"];
	        this.path_pattern = source["path_pattern"];
	        this.methods = source["methods"];
	        this.status_code = source["status_code"];
	        this.status_text = source["status_text"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.response_delay = source["response_delay"];
	        this.response_mode = source["response_mode"];
	        this.script_body = source["script_body"];
	        this.request_validation = this.convertValues(source["request_validation"], RequestValidation);
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
	export class AppConfig {
	    port: number;
	    responses?: MethodResponse[];
	    items?: ResponseItem[];
	    // Go type: time
	    last_modified?: any;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.responses = this.convertValues(source["responses"], MethodResponse);
	        this.items = this.convertValues(source["items"], ResponseItem);
	        this.last_modified = this.convertValues(source["last_modified"], null);
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
	
	export class RequestLog {
	    id: string;
	    // Go type: time
	    timestamp: any;
	    method: string;
	    path: string;
	    status_code: number;
	    source_ip: string;
	    headers?: Record<string, Array<string>>;
	    body?: string;
	    query_params?: Record<string, Array<string>>;
	    protocol?: string;
	    user_agent?: string;
	
	    static createFrom(source: any = {}) {
	        return new RequestLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.method = source["method"];
	        this.path = source["path"];
	        this.status_code = source["status_code"];
	        this.source_ip = source["source_ip"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.query_params = source["query_params"];
	        this.protocol = source["protocol"];
	        this.user_agent = source["user_agent"];
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

