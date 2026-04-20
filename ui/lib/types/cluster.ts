// Types for /api/cluster/status (spec 007).

export interface ClusterStatus {
	node_role: "standalone" | "leader" | "follower";
	hostname: string;
	version: string;
	started_at: string;
	uptime_seconds: number;
	pid: number;
	goroutines: number;
	process_memory_bytes: number;
}
