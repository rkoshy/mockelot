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
	
	export class CORSHeader {
	    name: string;
	    expression: string;
	
	    static createFrom(source: any = {}) {
	        return new CORSHeader(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.expression = source["expression"];
	    }
	}
	export class CORSConfig {
	    enabled: boolean;
	    mode?: string;
	    header_expressions?: CORSHeader[];
	    script?: string;
	    options_default_status?: number;
	
	    static createFrom(source: any = {}) {
	        return new CORSConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.mode = source["mode"];
	        this.header_expressions = this.convertValues(source["header_expressions"], CORSHeader);
	        this.script = source["script"];
	        this.options_default_status = source["options_default_status"];
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
	export class CertPaths {
	    ca_cert_path?: string;
	    ca_key_path?: string;
	    server_cert_path?: string;
	    server_key_path?: string;
	    server_bundle_path?: string;
	
	    static createFrom(source: any = {}) {
	        return new CertPaths(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ca_cert_path = source["ca_cert_path"];
	        this.ca_key_path = source["ca_key_path"];
	        this.server_cert_path = source["server_cert_path"];
	        this.server_key_path = source["server_key_path"];
	        this.server_bundle_path = source["server_bundle_path"];
	    }
	}
	export class ResponseGroup {
	    id?: string;
	    name: string;
	    expanded?: boolean;
	    enabled?: boolean;
	    use_global_cors?: boolean;
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
	        this.use_global_cors = source["use_global_cors"];
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
	    use_global_cors?: boolean;
	
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
	        this.use_global_cors = source["use_global_cors"];
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
	    http2_enabled?: boolean;
	    https_enabled?: boolean;
	    https_port?: number;
	    http_to_https_redirect?: boolean;
	    cert_mode?: string;
	    cert_paths?: CertPaths;
	    cert_names?: string[];
	    cors?: CORSConfig;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.responses = this.convertValues(source["responses"], MethodResponse);
	        this.items = this.convertValues(source["items"], ResponseItem);
	        this.last_modified = this.convertValues(source["last_modified"], null);
	        this.http2_enabled = source["http2_enabled"];
	        this.https_enabled = source["https_enabled"];
	        this.https_port = source["https_port"];
	        this.http_to_https_redirect = source["http_to_https_redirect"];
	        this.cert_mode = source["cert_mode"];
	        this.cert_paths = this.convertValues(source["cert_paths"], CertPaths);
	        this.cert_names = source["cert_names"];
	        this.cors = this.convertValues(source["cors"], CORSConfig);
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
	export class CACertInfo {
	    exists: boolean;
	    // Go type: time
	    generated?: any;
	
	    static createFrom(source: any = {}) {
	        return new CACertInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exists = source["exists"];
	        this.generated = this.convertValues(source["generated"], null);
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

