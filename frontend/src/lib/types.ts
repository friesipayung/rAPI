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
