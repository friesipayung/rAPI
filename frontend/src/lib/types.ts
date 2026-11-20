// Mirror of the Go model types (bindings are JS; keep the UI typed locally).

export interface VaultStatus {
	configured: boolean;
	path: string;
	name: string;
	valid: boolean;
	hasRemote: boolean;
	remoteUrl?: string;
	remotes?: string[];
}

export interface SyncStatus {
	state: 'idle' | 'pending' | 'syncing' | 'synced' | 'error' | 'conflict';
	detail?: string;
	at?: string;
}

export interface TreeNode {
	id: string;
	type: 'collection' | 'folder' | 'http';
	name: string;
	method?: string;
	url?: string;
	seq: number;
	docs?: boolean;
	examples?: number;
	envs?: string[];
	children?: TreeNode[];
}

export interface GitLogEntry {
	time: string;
	op: string;
	stderr: string;
	err?: string;
}

// ---------- phase 2: request / environment / run ----------

export interface KV {
	name: string;
	value?: string;
	type?: string; // query|path|header (params), query|header (apikey)
	description?: string;
	disabled?: boolean;
	secret?: boolean;
}

export interface Variable {
	name: string;
	value?: string;
	description?: string;
	secret?: boolean;
	disabled?: boolean;
}

export interface BasicAuth {
	username: string;
	password?: string;
}
export interface BearerAuth {
	token: string;
}
export interface ApiKeyAuth {
	key: string;
	value: string;
	in?: string;
}
export interface DigestAuth {
	username: string;
	password?: string;
}
export interface Auth {
	type?: string; // inherit|basic|bearer|apikey|digest
	basic?: BasicAuth;
	bearer?: BearerAuth;
	apikey?: ApiKeyAuth;
	digest?: DigestAuth;
}

export type BodyType = '' | 'json' | 'text' | 'xml' | 'html' | 'form-urlencoded' | 'multipart-form';

export interface RequestBody {
	type?: BodyType;
	data?: string;
	form?: KV[];
}

export interface Settings {
	encodeUrl?: boolean;
	timeout?: number;
	followRedirects?: boolean;
	maxRedirects?: number;
}

export interface HttpRequest {
	method: string;
	url: string;
	params?: KV[];
	headers?: KV[];
	auth?: Auth;
	body?: RequestBody;
}

export interface RequestDoc {
	info: { name: string; type?: string; seq?: number; description?: string };
	http: HttpRequest;
	settings?: Settings;
	docs?: string;
	// examples / runtime stay opaque (preserved server-side on save)
	examples?: unknown;
	runtime?: unknown;
}

export interface EnvironmentDoc {
	name: string;
	color?: string;
	description?: string;
	variables?: Variable[];
	extends?: string;
}

export interface CollectionDoc {
	opencollection?: string;
	info: { name: string; summary?: string; version?: string };
	config: { environments?: string[]; request?: unknown };
	docs?: string;
}

export interface FolderDoc {
	info: { name: string; type?: string; seq?: number; description?: string };
	docs?: string;
	request?: unknown;
}

export interface ResponseBody {
	type: string; // json|text|xml|html|binary
	data: string;
}

export interface RunResult {
	status: number;
	statusText: string;
	timeMs: number;
	sizeBytes: number;
	headers: KV[];
	body?: ResponseBody;
	request?: {
		method: string;
		url: string;
		params?: KV[];
		headers?: KV[];
		body?: RequestBody;
	};
	envUsed?: string;
	error?: string;
}

export const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'] as const;
export const BODY_TYPES: BodyType[] = ['json', 'text', 'xml', 'html', 'form-urlencoded', 'multipart-form'];
