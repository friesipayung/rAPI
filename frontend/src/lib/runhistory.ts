// In-memory run history, per request (max 20 entries, newest first).
// Session-scoped on purpose: responses never touch the git vault.
import type { RunResult } from './types';

export interface RunHistoryEntry {
	at: number; // epoch ms
	result: RunResult;
}

const LIMIT = 20;
const map = new Map<string, RunHistoryEntry[]>();

export function pushRun(requestID: string, result: RunResult) {
	let list = map.get(requestID);
	if (!list) {
		list = [];
		map.set(requestID, list);
	}
	list.unshift({ at: Date.now(), result });
	if (list.length > LIMIT) list.length = LIMIT;
}

export function getRuns(requestID: string): RunHistoryEntry[] {
	return map.get(requestID) ?? [];
}
