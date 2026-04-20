// Enterprise RBAC view (US2, T032).
//
// Roles list + create/edit/delete with a full permission matrix over
// the 24 RbacResource × 6 RbacOperation cells. Wraps the
// /api/rbac/roles handler via RTK Query.

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
import { Checkbox } from "@/components/ui/checkbox";
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
	getErrorMessage,
	useCreateRoleMutation,
	useDeleteRoleMutation,
	useGetRbacMetaQuery,
	useGetRolesQuery,
	useUpdateRoleMutation,
} from "@/lib/store";
import type { EnterpriseRole, RoleScopeMap } from "@/lib/types/enterprise";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Edit, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

function resourceDisplay(r: string): string {
	return r.replace(/([A-Z])/g, " $1").trim();
}

function scopeCount(scopes: RoleScopeMap | null | undefined): number {
	if (!scopes) return 0;
	return Object.values(scopes).reduce((sum, ops) => sum + ops.length, 0);
}

interface RoleEditorDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	role: EnterpriseRole | null;
	resources: string[];
	operations: string[];
}

function RoleEditorDialog({ open, onOpenChange, role, resources, operations }: RoleEditorDialogProps) {
	const [name, setName] = useState("");
	const [scopes, setScopes] = useState<RoleScopeMap>({});

	const [createRole, { isLoading: isCreating }] = useCreateRoleMutation();
	const [updateRole, { isLoading: isUpdating }] = useUpdateRoleMutation();
	const isSaving = isCreating || isUpdating;

	useEffect(() => {
		if (!open) return;
		setName(role?.name ?? "");
		if (role?.scopes) {
			const normalized: RoleScopeMap = {};
			if (role.scopes["*"]) {
				for (const r of resources) normalized[r] = [...operations];
			} else {
				for (const r of resources) normalized[r] = role.scopes[r] ? [...role.scopes[r]] : [];
			}
			setScopes(normalized);
		} else {
			const empty: RoleScopeMap = {};
			for (const r of resources) empty[r] = [];
			setScopes(empty);
		}
	}, [open, role, resources, operations]);

	const toggleCell = (resource: string, op: string) => {
		setScopes((prev) => {
			const current = prev[resource] ?? [];
			const next = current.includes(op) ? current.filter((x) => x !== op) : [...current, op];
			return { ...prev, [resource]: next };
		});
	};

	const toggleRow = (resource: string) => {
		setScopes((prev) => {
			const current = prev[resource] ?? [];
			const allSelected = current.length === operations.length;
			return { ...prev, [resource]: allSelected ? [] : [...operations] };
		});
	};

	const handleSave = async () => {
		const trimmed = name.trim();
		if (!trimmed) {
			toast.error("Role name is required");
			return;
		}
		const clean: RoleScopeMap = {};
		for (const [r, ops] of Object.entries(scopes)) if (ops.length > 0) clean[r] = ops;

		try {
			if (role) {
				await updateRole({ id: role.id, data: { name: trimmed, scopes: clean } }).unwrap();
				toast.success("Role updated");
			} else {
				await createRole({ name: trimmed, scopes: clean }).unwrap();
				toast.success("Role created");
			}
			onOpenChange(false);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="max-h-[90vh] max-w-4xl overflow-y-auto" data-testid="role-editor-dialog">
				<DialogHeader>
					<DialogTitle>{role ? "Edit role" : "Create role"}</DialogTitle>
					<DialogDescription>
						Grant operations per resource. Each row is a resource; each column is an operation.
					</DialogDescription>
				</DialogHeader>

				<div className="space-y-2">
					<Label htmlFor="role-name">Name</Label>
					<Input
						id="role-name"
						data-testid="role-name-input"
						value={name}
						onChange={(e) => setName(e.target.value)}
						placeholder="e.g. ReadOnly Analyst"
					/>
				</div>

				<div className="rounded-md border">
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead className="w-[220px]">Resource</TableHead>
								{operations.map((op) => (
									<TableHead key={op} className="text-center">{op}</TableHead>
								))}
							</TableRow>
						</TableHeader>
						<TableBody>
							{resources.map((resource) => {
								const row = scopes[resource] ?? [];
								const all = row.length === operations.length;
								return (
									<TableRow key={resource} data-testid={`role-resource-row-${resource}`}>
										<TableCell>
											<button
												type="button"
												className="text-left font-medium hover:underline"
												onClick={() => toggleRow(resource)}
												data-testid={`role-row-toggle-${resource}`}
											>
												{resourceDisplay(resource)}
												{all && <Badge variant="secondary" className="ml-2">all</Badge>}
											</button>
										</TableCell>
										{operations.map((op) => (
											<TableCell key={op} className="text-center">
												<Checkbox
													checked={row.includes(op)}
													onCheckedChange={() => toggleCell(resource, op)}
													data-testid={`role-cell-${resource}-${op}`}
												/>
											</TableCell>
										))}
									</TableRow>
								);
							})}
						</TableBody>
					</Table>
				</div>

				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)} disabled={isSaving}>
						Cancel
					</Button>
					<Button onClick={handleSave} disabled={isSaving} data-testid="role-save-button">
						{isSaving ? "Saving…" : role ? "Save changes" : "Create role"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

export default function RBACView() {
	const hasView = useRbac(RbacResource.RBAC, RbacOperation.View);
	const hasCreate = useRbac(RbacResource.RBAC, RbacOperation.Create);
	const hasUpdate = useRbac(RbacResource.RBAC, RbacOperation.Update);
	const hasDelete = useRbac(RbacResource.RBAC, RbacOperation.Delete);

	const { data: meta, isLoading: metaLoading } = useGetRbacMetaQuery(undefined, { skip: !hasView });
	const { data: rolesData, isLoading: rolesLoading, error } = useGetRolesQuery(undefined, { skip: !hasView });
	const [deleteRole, { isLoading: isDeleting }] = useDeleteRoleMutation();

	const [editorOpen, setEditorOpen] = useState(false);
	const [editing, setEditing] = useState<EnterpriseRole | null>(null);
	const [confirmDelete, setConfirmDelete] = useState<EnterpriseRole | null>(null);

	const roles = useMemo(() => rolesData?.roles ?? [], [rolesData]);

	useEffect(() => {
		if (error) toast.error(`Failed to load roles: ${getErrorMessage(error)}`);
	}, [error]);

	if (!hasView) return <NoPermissionView entity="roles and permissions" />;
	if (metaLoading || rolesLoading) return <FullPageLoader />;

	const resources = meta?.resources ?? [];
	const operations = meta?.operations ?? [];

	const openCreate = () => {
		setEditing(null);
		setEditorOpen(true);
	};
	const openEdit = (r: EnterpriseRole) => {
		setEditing(r);
		setEditorOpen(true);
	};

	const handleConfirmDelete = async () => {
		if (!confirmDelete) return;
		try {
			await deleteRole(confirmDelete.id).unwrap();
			toast.success(`Role "${confirmDelete.name}" deleted`);
			setConfirmDelete(null);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	return (
		<div className="space-y-6" data-testid="rbac-view">
			<div className="flex items-center justify-between">
				<div>
					<h2 className="text-2xl font-semibold">Roles &amp; permissions</h2>
					<p className="text-muted-foreground text-sm">
						Define what members can do. {resources.length} resources × {operations.length} operations.
					</p>
				</div>
				{hasCreate && (
					<Button onClick={openCreate} data-testid="create-role-button">
						<Plus className="mr-2 h-4 w-4" /> Create role
					</Button>
				)}
			</div>

			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead>Type</TableHead>
							<TableHead>Scopes</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{roles.length === 0 ? (
							<TableRow>
								<TableCell colSpan={4} className="text-muted-foreground py-10 text-center">
									No roles yet.
								</TableCell>
							</TableRow>
						) : (
							roles.map((role) => (
								<TableRow key={role.id} data-testid={`role-row-${role.name}`}>
									<TableCell className="font-medium">{role.name}</TableCell>
									<TableCell>
										{role.is_builtin ? <Badge>Built-in</Badge> : <Badge variant="secondary">Custom</Badge>}
									</TableCell>
									<TableCell>{scopeCount(role.scopes)} permission(s)</TableCell>
									<TableCell className="text-right">
										{hasUpdate && (
											<Button
												variant="ghost"
												size="sm"
												onClick={() => openEdit(role)}
												disabled={role.is_builtin}
												data-testid={`edit-role-${role.name}`}
											>
												<Edit className="h-4 w-4" />
											</Button>
										)}
										{hasDelete && !role.is_builtin && (
											<Button
												variant="ghost"
												size="sm"
												onClick={() => setConfirmDelete(role)}
												data-testid={`delete-role-${role.name}`}
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

			<RoleEditorDialog
				open={editorOpen}
				onOpenChange={setEditorOpen}
				role={editing}
				resources={resources}
				operations={operations}
			/>

			<AlertDialog open={!!confirmDelete} onOpenChange={(o) => !o && setConfirmDelete(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete role?</AlertDialogTitle>
						<AlertDialogDescription>
							Delete the role &quot;{confirmDelete?.name}&quot;? Users currently assigned to it will lose
							its permissions. This cannot be undone.
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
