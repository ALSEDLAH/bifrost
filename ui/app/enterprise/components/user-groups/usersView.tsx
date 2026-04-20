// Enterprise users view (US2, T033).
//
// Lists enterprise users (ent_users) with their role assignments and
// exposes invite/update/delete plus role-assignment management.
// Wraps the /api/rbac/users + /api/rbac/assignments handlers.

import FullPageLoader from "@/components/fullPageLoader";
import { NoPermissionView } from "@/components/noPermissionView";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alertDialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
	getErrorMessage,
	useAssignRoleMutation,
	useCreateEnterpriseUserMutation,
	useDeleteEnterpriseUserMutation,
	useGetEnterpriseUsersQuery,
	useGetRolesQuery,
	useUnassignRoleMutation,
} from "@/lib/store";
import type { EnterpriseRole, EnterpriseUser } from "@/lib/types/enterprise";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Plus, Trash2, UserPlus } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

function statusBadge(status: string) {
	if (status === "active") return <Badge>Active</Badge>;
	if (status === "suspended") return <Badge variant="destructive">Suspended</Badge>;
	return <Badge variant="secondary">Pending</Badge>;
}

interface InviteUserDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

function InviteUserDialog({ open, onOpenChange }: InviteUserDialogProps) {
	const [email, setEmail] = useState("");
	const [displayName, setDisplayName] = useState("");
	const [createUser, { isLoading }] = useCreateEnterpriseUserMutation();

	useEffect(() => {
		if (!open) {
			setEmail("");
			setDisplayName("");
		}
	}, [open]);

	const handleSave = async () => {
		if (!email.trim()) {
			toast.error("Email is required");
			return;
		}
		try {
			await createUser({ email: email.trim(), display_name: displayName.trim() || undefined }).unwrap();
			toast.success("User invited");
			onOpenChange(false);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent data-testid="invite-user-dialog">
				<DialogHeader>
					<DialogTitle>Invite user</DialogTitle>
					<DialogDescription>
						Creates a pending user record. The user is activated on first SSO login.
					</DialogDescription>
				</DialogHeader>
				<div className="space-y-4">
					<div className="space-y-2">
						<Label htmlFor="invite-email">Email</Label>
						<Input
							id="invite-email"
							data-testid="invite-email-input"
							value={email}
							onChange={(e) => setEmail(e.target.value)}
							placeholder="user@example.com"
							type="email"
						/>
					</div>
					<div className="space-y-2">
						<Label htmlFor="invite-name">Display name (optional)</Label>
						<Input
							id="invite-name"
							data-testid="invite-name-input"
							value={displayName}
							onChange={(e) => setDisplayName(e.target.value)}
							placeholder="Jane Doe"
						/>
					</div>
				</div>
				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)} disabled={isLoading}>
						Cancel
					</Button>
					<Button onClick={handleSave} disabled={isLoading} data-testid="invite-user-submit">
						{isLoading ? "Inviting…" : "Invite"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

interface AssignRoleDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	user: EnterpriseUser | null;
	roles: EnterpriseRole[];
}

function AssignRoleDialog({ open, onOpenChange, user, roles }: AssignRoleDialogProps) {
	const [roleId, setRoleId] = useState<string>("");
	const [assignRole, { isLoading: isAssigning }] = useAssignRoleMutation();
	const [unassignRole, { isLoading: isUnassigning }] = useUnassignRoleMutation();

	useEffect(() => {
		if (!open) setRoleId("");
	}, [open]);

	const rolesById = useMemo(() => Object.fromEntries(roles.map((r) => [r.id, r])), [roles]);

	const handleAssign = async () => {
		if (!user) return;
		if (!roleId) {
			toast.error("Select a role");
			return;
		}
		try {
			await assignRole({ user_id: user.id, role_id: roleId }).unwrap();
			toast.success("Role assigned");
			setRoleId("");
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	const handleUnassign = async (assignmentId: string) => {
		try {
			await unassignRole(assignmentId).unwrap();
			toast.success("Role removed");
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent data-testid="assign-role-dialog">
				<DialogHeader>
					<DialogTitle>Roles for {user?.email}</DialogTitle>
					<DialogDescription>Assign roles to grant this user their permissions.</DialogDescription>
				</DialogHeader>

				<div className="space-y-4">
					<div>
						<div className="mb-2 text-sm font-medium">Current roles</div>
						{user?.assignments && user.assignments.length > 0 ? (
							<ul className="space-y-2">
								{user.assignments.map((a) => {
									const r = rolesById[a.role_id];
									return (
										<li
											key={a.id}
											className="flex items-center justify-between rounded-md border px-3 py-2"
											data-testid={`assignment-row-${a.id}`}
										>
											<div>
												<div className="font-medium">{r?.name ?? a.role_id}</div>
												{a.workspace_id && (
													<div className="text-muted-foreground text-xs">Workspace {a.workspace_id}</div>
												)}
											</div>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => handleUnassign(a.id)}
												disabled={isUnassigning}
												data-testid={`unassign-${a.id}`}
											>
												<Trash2 className="text-destructive h-4 w-4" />
											</Button>
										</li>
									);
								})}
							</ul>
						) : (
							<div className="text-muted-foreground text-sm">No roles assigned.</div>
						)}
					</div>

					<div className="space-y-2">
						<Label>Add role</Label>
						<div className="flex gap-2">
							<Select value={roleId} onValueChange={setRoleId}>
								<SelectTrigger className="flex-1" data-testid="assign-role-select">
									<SelectValue placeholder="Select a role" />
								</SelectTrigger>
								<SelectContent>
									{roles.map((r) => (
										<SelectItem key={r.id} value={r.id}>
											{r.name} {r.is_builtin && <span className="text-muted-foreground">(built-in)</span>}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
							<Button onClick={handleAssign} disabled={isAssigning} data-testid="assign-role-submit">
								<Plus className="mr-2 h-4 w-4" /> Assign
							</Button>
						</div>
					</div>
				</div>

				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>Close</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

export default function UsersView() {
	const hasView = useRbac(RbacResource.Users, RbacOperation.View);
	const hasCreate = useRbac(RbacResource.Users, RbacOperation.Create);
	const hasUpdate = useRbac(RbacResource.Users, RbacOperation.Update);
	const hasDelete = useRbac(RbacResource.Users, RbacOperation.Delete);

	const { data: usersData, isLoading: usersLoading, error } = useGetEnterpriseUsersQuery(undefined, { skip: !hasView });
	const { data: rolesData, isLoading: rolesLoading } = useGetRolesQuery(undefined, { skip: !hasView });
	const [deleteUser, { isLoading: isDeleting }] = useDeleteEnterpriseUserMutation();

	const [inviteOpen, setInviteOpen] = useState(false);
	const [assignUser, setAssignUser] = useState<EnterpriseUser | null>(null);
	const [confirmDelete, setConfirmDelete] = useState<EnterpriseUser | null>(null);

	const users = useMemo(() => usersData?.users ?? [], [usersData]);
	const roles = useMemo(() => rolesData?.roles ?? [], [rolesData]);
	const rolesById = useMemo(() => Object.fromEntries(roles.map((r) => [r.id, r])), [roles]);

	useEffect(() => {
		if (error) toast.error(`Failed to load users: ${getErrorMessage(error)}`);
	}, [error]);

	// Keep the currently-open assignment dialog in sync with refetched data.
	const refreshedAssignUser = useMemo(() => {
		if (!assignUser) return null;
		return users.find((u) => u.id === assignUser.id) ?? assignUser;
	}, [users, assignUser]);

	if (!hasView) return <NoPermissionView entity="users" />;
	if (usersLoading || rolesLoading) return <FullPageLoader />;

	const handleConfirmDelete = async () => {
		if (!confirmDelete) return;
		try {
			await deleteUser(confirmDelete.id).unwrap();
			toast.success(`User "${confirmDelete.email}" deleted`);
			setConfirmDelete(null);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	return (
		<div className="space-y-6" data-testid="users-view">
			<div className="flex items-center justify-between">
				<div>
					<h2 className="text-2xl font-semibold">Users</h2>
					<p className="text-muted-foreground text-sm">
						Invite members and assign roles. {users.length} user(s).
					</p>
				</div>
				{hasCreate && (
					<Button onClick={() => setInviteOpen(true)} data-testid="invite-user-button">
						<UserPlus className="mr-2 h-4 w-4" /> Invite user
					</Button>
				)}
			</div>

			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Email</TableHead>
							<TableHead>Name</TableHead>
							<TableHead>Status</TableHead>
							<TableHead>Roles</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{users.length === 0 ? (
							<TableRow>
								<TableCell colSpan={5} className="text-muted-foreground py-10 text-center">
									No users yet.
								</TableCell>
							</TableRow>
						) : (
							users.map((u) => (
								<TableRow key={u.id} data-testid={`user-row-${u.email}`}>
									<TableCell className="font-medium">{u.email}</TableCell>
									<TableCell>{u.display_name || "—"}</TableCell>
									<TableCell>{statusBadge(u.status)}</TableCell>
									<TableCell>
										<div className="flex flex-wrap gap-1">
											{u.assignments && u.assignments.length > 0 ? (
												u.assignments.map((a) => (
													<Badge key={a.id} variant="secondary">
														{rolesById[a.role_id]?.name ?? "Unknown"}
													</Badge>
												))
											) : (
												<span className="text-muted-foreground text-sm">—</span>
											)}
										</div>
									</TableCell>
									<TableCell className="text-right">
										{hasUpdate && (
											<Button
												variant="ghost"
												size="sm"
												onClick={() => setAssignUser(u)}
												data-testid={`assign-role-${u.email}`}
											>
												Manage roles
											</Button>
										)}
										{hasDelete && (
											<Button
												variant="ghost"
												size="sm"
												onClick={() => setConfirmDelete(u)}
												data-testid={`delete-user-${u.email}`}
											>
												<Trash2 className="text-destructive h-4 w-4" />
											</Button>
										)}
									</TableCell>
								</TableRow>
							))
						)}
					</TableBody>
				</Table>
			</div>

			<InviteUserDialog open={inviteOpen} onOpenChange={setInviteOpen} />

			<AssignRoleDialog
				open={!!assignUser}
				onOpenChange={(o) => !o && setAssignUser(null)}
				user={refreshedAssignUser}
				roles={roles}
			/>

			<AlertDialog open={!!confirmDelete} onOpenChange={(o) => !o && setConfirmDelete(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete user?</AlertDialogTitle>
						<AlertDialogDescription>
							Delete user &quot;{confirmDelete?.email}&quot;? All role assignments for this user will be
							removed. This cannot be undone.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
						<AlertDialogAction onClick={handleConfirmDelete} disabled={isDeleting}>
							{isDeleting ? "Deleting…" : "Delete"}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
}
