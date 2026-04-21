// Types for /api/reports/* (spec 019).

export interface ActivityBucket {
	action: string;
	outcome: string;
	count: number;
}

export interface AdminActivityReport {
	window_days: number;
	since: string;
	buckets: ActivityBucket[];
}

export interface AccessControlReport {
	window_days: number;
	since: string;
	role_changes: number;
	role_assignments: number;
	user_creates: number;
	user_deletes: number;
	key_rotations: number;
}
