export namespace main {
	
	export class Event {
	    source: string;
	    data: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.data = source["data"];
	    }
	}
	export class ScriptErrorLog {
	    // Go type: time
	    timestamp: any;
	    error: string;
	    response_id: string;
	    path: string;
	    method: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptErrorLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.error = source["error"];
	        this.response_id = source["response_id"];
	        this.path = source["path"];
	        this.method = source["method"];
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
	export class EnvironmentVar {
	    name: string;
	    value?: string;
	    expression?: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentVar(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	        this.expression = source["expression"];
	    }
	}
	export class VolumeMapping {
	    host_path: string;
	    container_path: string;
	    read_only: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VolumeMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host_path = source["host_path"];
	        this.container_path = source["container_path"];
	        this.read_only = source["read_only"];
	    }
	}
	export class ContainerConfig {
	    proxy_config: ProxyConfig;
	    image_name: string;
	    container_port: number;
	    exposed_ports?: string[];
	    pull_on_startup: boolean;
	    restart_policy?: string;
	    volumes?: VolumeMapping[];
	    environment?: EnvironmentVar[];
	    host_networking?: boolean;
	    docker_socket_access?: boolean;
	    restart_on_server_start?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContainerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxy_config = this.convertValues(source["proxy_config"], ProxyConfig);
	        this.image_name = source["image_name"];
	        this.container_port = source["container_port"];
	        this.exposed_ports = source["exposed_ports"];
	        this.pull_on_startup = source["pull_on_startup"];
	        this.restart_policy = source["restart_policy"];
	        this.volumes = this.convertValues(source["volumes"], VolumeMapping);
	        this.environment = this.convertValues(source["environment"], EnvironmentVar);
	        this.host_networking = source["host_networking"];
	        this.docker_socket_access = source["docker_socket_access"];
	        this.restart_on_server_start = source["restart_on_server_start"];
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
	export class StatusTranslation {
	    from_pattern: string;
	    to_code: number;
	
	    static createFrom(source: any = {}) {
	        return new StatusTranslation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from_pattern = source["from_pattern"];
	        this.to_code = source["to_code"];
	    }
	}
	export class HeaderManipulation {
	    name: string;
	    mode: string;
	    value?: string;
	    expression?: string;
	
	    static createFrom(source: any = {}) {
	        return new HeaderManipulation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.value = source["value"];
	        this.expression = source["expression"];
	    }
	}
	export class ProxyConfig {
	    backend_url: string;
	    timeout_seconds: number;
	    inbound_headers?: HeaderManipulation[];
	    outbound_headers?: HeaderManipulation[];
	    status_passthrough: boolean;
	    status_translation?: StatusTranslation[];
	    body_transform?: string;
	    health_check_enabled: boolean;
	    health_check_interval: number;
	    health_check_path?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backend_url = source["backend_url"];
	        this.timeout_seconds = source["timeout_seconds"];
	        this.inbound_headers = this.convertValues(source["inbound_headers"], HeaderManipulation);
	        this.outbound_headers = this.convertValues(source["outbound_headers"], HeaderManipulation);
	        this.status_passthrough = source["status_passthrough"];
	        this.status_translation = this.convertValues(source["status_translation"], StatusTranslation);
	        this.body_transform = source["body_transform"];
	        this.health_check_enabled = source["health_check_enabled"];
	        this.health_check_interval = source["health_check_interval"];
	        this.health_check_path = source["health_check_path"];
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
	export class Endpoint {
	    id: string;
	    name: string;
	    path_prefix: string;
	    translation_mode: string;
	    translate_pattern?: string;
	    translate_replace?: string;
	    enabled?: boolean;
	    type: string;
	    items?: ResponseItem[];
	    proxy_config?: ProxyConfig;
	    container_config?: ContainerConfig;
	
	    static createFrom(source: any = {}) {
	        return new Endpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path_prefix = source["path_prefix"];
	        this.translation_mode = source["translation_mode"];
	        this.translate_pattern = source["translate_pattern"];
	        this.translate_replace = source["translate_replace"];
	        this.enabled = source["enabled"];
	        this.type = source["type"];
	        this.items = this.convertValues(source["items"], ResponseItem);
	        this.proxy_config = this.convertValues(source["proxy_config"], ProxyConfig);
	        this.container_config = this.convertValues(source["container_config"], ContainerConfig);
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
	export class HeaderValidation {
	    name: string;
	    mode?: string;
	    value?: string;
	    pattern?: string;
	    expression?: string;
	    required?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HeaderValidation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.value = source["value"];
	        this.pattern = source["pattern"];
	        this.expression = source["expression"];
	        this.required = source["required"];
	    }
	}
	export class RequestValidation {
	    mode?: string;
	    pattern?: string;
	    match_type?: string;
	    script?: string;
	    headers?: HeaderValidation[];
	
	    static createFrom(source: any = {}) {
	        return new RequestValidation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.pattern = source["pattern"];
	        this.match_type = source["match_type"];
	        this.script = source["script"];
	        this.headers = this.convertValues(source["headers"], HeaderValidation);
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
	    endpoints?: Endpoint[];
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
	    container_log_line_limit?: number;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.responses = this.convertValues(source["responses"], MethodResponse);
	        this.items = this.convertValues(source["items"], ResponseItem);
	        this.endpoints = this.convertValues(source["endpoints"], Endpoint);
	        this.last_modified = this.convertValues(source["last_modified"], null);
	        this.http2_enabled = source["http2_enabled"];
	        this.https_enabled = source["https_enabled"];
	        this.https_port = source["https_port"];
	        this.http_to_https_redirect = source["http_to_https_redirect"];
	        this.cert_mode = source["cert_mode"];
	        this.cert_paths = this.convertValues(source["cert_paths"], CertPaths);
	        this.cert_names = source["cert_names"];
	        this.cors = this.convertValues(source["cors"], CORSConfig);
	        this.container_log_line_limit = source["container_log_line_limit"];
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
	    generated?: string;
	
	    static createFrom(source: any = {}) {
	        return new CACertInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exists = source["exists"];
	        this.generated = source["generated"];
	    }
	}
	
	
	
	
	export class ContainerStats {
	    endpoint_id: string;
	    cpu_percent: number;
	    memory_usage_mb: number;
	    memory_limit_mb: number;
	    memory_percent: number;
	    network_rx_bytes: number;
	    network_tx_bytes: number;
	    block_read_bytes: number;
	    block_write_bytes: number;
	    pids: number;
	    last_check: string;
	
	    static createFrom(source: any = {}) {
	        return new ContainerStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint_id = source["endpoint_id"];
	        this.cpu_percent = source["cpu_percent"];
	        this.memory_usage_mb = source["memory_usage_mb"];
	        this.memory_limit_mb = source["memory_limit_mb"];
	        this.memory_percent = source["memory_percent"];
	        this.network_rx_bytes = source["network_rx_bytes"];
	        this.network_tx_bytes = source["network_tx_bytes"];
	        this.block_read_bytes = source["block_read_bytes"];
	        this.block_write_bytes = source["block_write_bytes"];
	        this.pids = source["pids"];
	        this.last_check = source["last_check"];
	    }
	}
	export class ContainerStatus {
	    endpoint_id: string;
	    container_id: string;
	    running: boolean;
	    status: string;
	    gone: boolean;
	    last_check: string;
	
	    static createFrom(source: any = {}) {
	        return new ContainerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint_id = source["endpoint_id"];
	        this.container_id = source["container_id"];
	        this.running = source["running"];
	        this.status = source["status"];
	        this.gone = source["gone"];
	        this.last_check = source["last_check"];
	    }
	}
	export class DockerImageInfo {
	    image_name: string;
	    exposed_ports: string[];
	    volumes: string[];
	    environment: Record<string, string>;
	    working_dir?: string;
	    entrypoint?: string[];
	    cmd?: string[];
	    labels?: Record<string, string>;
	    suggested_health_check_path?: string;
	    is_http_service: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DockerImageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.image_name = source["image_name"];
	        this.exposed_ports = source["exposed_ports"];
	        this.volumes = source["volumes"];
	        this.environment = source["environment"];
	        this.working_dir = source["working_dir"];
	        this.entrypoint = source["entrypoint"];
	        this.cmd = source["cmd"];
	        this.labels = source["labels"];
	        this.suggested_health_check_path = source["suggested_health_check_path"];
	        this.is_http_service = source["is_http_service"];
	    }
	}
	
	
	
	
	export class HealthStatus {
	    endpoint_id: string;
	    healthy: boolean;
	    last_check: string;
	    error_message?: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint_id = source["endpoint_id"];
	        this.healthy = source["healthy"];
	        this.last_check = source["last_check"];
	        this.error_message = source["error_message"];
	    }
	}
	
	
	export class RecentFile {
	    path: string;
	    // Go type: time
	    last_accessed: any;
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RecentFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.last_accessed = this.convertValues(source["last_accessed"], null);
	        this.exists = source["exists"];
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
	    timestamp: string;
	    endpoint_id?: string;
	    // Go type: struct { Method string "json:\"method\""; FullURL string "json:\"full_url\""; Path string "json:\"path\""; QueryParams map[string][]string "json:\"query_params,omitempty\""; Headers map[string][]string "json:\"headers,omitempty\""; Body string "json:\"body,omitempty\""; Protocol string "json:\"protocol,omitempty\""; SourceIP string "json:\"source_ip\""; UserAgent string "json:\"user_agent,omitempty\"" }
	    client_request: any;
	    // Go type: struct { StatusCode int "json:\"status_code\""; StatusText string "json:\"status_text,omitempty\""; Headers map[string][]string "json:\"headers,omitempty\""; Body string "json:\"body,omitempty\""; DelayMs *int64 "json:\"delay_ms,omitempty\""; RTTMs *int64 "json:\"rtt_ms,omitempty\"" }
	    client_response: any;
	    // Go type: struct { Method string "json:\"method\""; FullURL string "json:\"full_url\""; Path string "json:\"path\""; QueryParams map[string][]string "json:\"query_params,omitempty\""; Headers map[string][]string "json:\"headers,omitempty\""; Body string "json:\"body,omitempty\"" }
	    backend_request?: any;
	    // Go type: struct { StatusCode int "json:\"status_code\""; StatusText string "json:\"status_text,omitempty\""; Headers map[string][]string "json:\"headers,omitempty\""; Body string "json:\"body,omitempty\""; DelayMs *int64 "json:\"delay_ms,omitempty\""; RTTMs *int64 "json:\"rtt_ms,omitempty\"" }
	    backend_response?: any;
	
	    static createFrom(source: any = {}) {
	        return new RequestLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.endpoint_id = source["endpoint_id"];
	        this.client_request = this.convertValues(source["client_request"], Object);
	        this.client_response = this.convertValues(source["client_response"], Object);
	        this.backend_request = this.convertValues(source["backend_request"], Object);
	        this.backend_response = this.convertValues(source["backend_response"], Object);
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
	export class RequestLogSummary {
	    id: string;
	    timestamp: string;
	    endpoint_id?: string;
	    method: string;
	    path: string;
	    source_ip: string;
	    client_status: number;
	    backend_status: number;
	    client_rtt?: number;
	    backend_rtt?: number;
	    has_backend: boolean;
	    client_body_size: number;
	    pending: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RequestLogSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.endpoint_id = source["endpoint_id"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.source_ip = source["source_ip"];
	        this.client_status = source["client_status"];
	        this.backend_status = source["backend_status"];
	        this.client_rtt = source["client_rtt"];
	        this.backend_rtt = source["backend_rtt"];
	        this.has_backend = source["has_backend"];
	        this.client_body_size = source["client_body_size"];
	        this.pending = source["pending"];
	    }
	}
	
	
	
	

}

