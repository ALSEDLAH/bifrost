// MCP Tool Groups view (spec 005).
//
// Admins create named tool groups as labels for subsets of MCP tools.
// v1 is CRUD only; VK gating is a follow-up spec.

import { NoPermissionView } from "@/components/noPermissionView";
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
import { Textarea } from "@/components/ui/textarea";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import {
	getErrorMessage,
	useCreateMCPToolGroupMutation,
	useDeleteMCPToolGroupMutation,
	useGetMCPClientsQuery,
	useGetMCPToolGroupsQuery,
	useUpdateMCPToolGroupMutation,
} from "@/lib/store/apis";
import { parseToolRefs, type MCPToolGroup, type MCPToolRef } from "@/lib/types/mcpToolGroups";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";

type DialogMode = { kind: "closed" } | { kind: "new" } | { kind: "edit"; group: MCPToolGroup };

function refKey(r: MCPToolRef) {
	return `${r.mcp_client_id}::${r.tool_name}`;
}

export default function MCPToolGroupsView() {
	const hasView = useRbac(RbacResource.MCPToolGroups, RbacOperation.View);
	const hasCreate = useRbac(RbacResource.MCPToolGroups, RbacOperation.Create);
	const hasUpdate = useRbac(RbacResource.MCPToolGroups, RbacOperation.Update);
	const hasDelete = useRbac(RbacResource.MCPToolGroups, RbacOperation.Delete);

	const { data: groupsData, isLoading } = useGetMCPToolGroupsQuery(undefined, { skip: !hasView });
	const { data: clientsData } = useGetMCPClientsQuery({ limit: 1000 } as never, { skip: !hasView });
	const [createGroup, { isLoading: isCreating }] = useCreateMCPToolGroupMutation();
	const [updateGroup, { isLoading: isUpdating }] = useUpdateMCPToolGroupMutation();
	const [deleteGroup] = useDeleteMCPToolGroupMutation();

	const [mode, setMode] = useState<DialogMode>({ kind: "closed" });
	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [selected, setSelected] = useState<Record<string, MCPToolRef>>({});

	const groups = useMemo(() => groupsData?.groups ?? [], [groupsData]);
	const clients = useMemo(() => clientsData?.clients ?? [], [clientsData]);

	if (!hasView) return <NoPermissionView entity="MCP tool groups" />;

	function resetDialog() {
		setMode({ kind: "closed" });
		setName("");
		setDescription("");
		setSelected({});
	}

	function openNew() {
		setName("");
		setDescription("");
		setSelected({});
		setMode({ kind: "new" });
	}

	function openEdit(g: MCPToolGroup) {
		setName(g.name);
		setDescription(g.description);
		const refs = parseToolRefs(g);
		const next: Record<string, MCPToolRef> = {};
		for (const r of refs) next[refKey(r)] = r;
		setSelected(next);
		setMode({ kind: "edit", group: g });
	}

	function toggleRef(r: MCPToolRef) {
		setSelected((prev) => {
			const next = { ...prev };
			const k = refKey(r);
			if (next[k]) delete next[k];
			else next[k] = r;
			return next;
		});
	}

	async function handleSave() {
		if (!name.trim()) {
			toast.error("Name is required");
			return;
		}
		const tools = Object.values(selected);
		try {
			if (mode.kind === "new") {
				await createGroup({ name: name.trim(), description: description.trim(), tools }).unwrap();
				toast.success("Tool group created");
			} else if (mode.kind === "edit") {
				await updateGroup({
					id: mode.group.id,
					patch: { name: name.trim(), description: description.trim(), tools },
				}).unwrap();
				toast.success("Tool group updated");
			}
			resetDialog();
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function handleDelete(g: MCPToolGroup) {
		if (!confirm(`Delete tool group "${g.name}"?`)) return;
		try {
			await deleteGroup({ id: g.id }).unwrap();
			toast.success("Tool group deleted");
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	return (
		<div className="mx-auto w-full max-w-5xl space-y-4" data-testid="mcp-tool-groups-view">
			<div className="flex items-start justify-between">
				<div>
					<h2 className="text-lg font-semibold tracking-tight">MCP Tool Groups</h2>
					<p className="text-muted-foreground text-sm">
						Organize MCP tools into named groups for discoverability. Virtual-key scoping is a follow-up feature.
					</p>
				</div>
				{hasCreate && (
					<Button onClick={openNew} data-testid="mcp-tool-groups-new">
						<Plus className="mr-1 h-4 w-4" /> New group
					</Button>
				)}
			</div>

			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead>Description</TableHead>
							<TableHead>Tools</TableHead>
							<TableHead>Updated</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{isLoading ? (
							<TableRow>
								<TableCell colSpan={5} className="text-muted-foreground py-10 text-center">
									Loading…
								</TableCell>
							</TableRow>
						) : groups.length === 0 ? (
							<TableRow>
								<TableCell
									colSpan={5}
									className="text-muted-foreground py-10 text-center"
									data-testid="mcp-tool-groups-empty"
								>
									No tool groups configured.
								</TableCell>
							</TableRow>
						) : (
							groups.map((g) => {
								const refs = parseToolRefs(g);
								return (
									<TableRow key={g.id} data-testid={`mcp-tool-groups-row-${g.id}`}>
										<TableCell className="font-medium">{g.name}</TableCell>
										<TableCell className="text-muted-foreground max-w-[280px] truncate text-sm">
											{g.description || "—"}
										</TableCell>
										<TableCell>
											<Badge variant="outline">{refs.length} tools</Badge>
										</TableCell>
										<TableCell className="text-muted-foreground text-xs">
											{new Date(g.updated_at).toLocaleString()}
										</TableCell>
										<TableCell className="flex justify-end gap-2">
											<Button
												variant="outline"
												size="sm"
												disabled={!hasUpdate}
												onClick={() => openEdit(g)}
												data-testid={`mcp-tool-groups-edit-${g.id}`}
											>
												<Pencil className="h-3 w-3" />
											</Button>
											<Button
												variant="outline"
												size="sm"
												disabled={!hasDelete}
												onClick={() => handleDelete(g)}
												data-testid={`mcp-tool-groups-delete-${g.id}`}
											>
												<Trash2 className="h-3 w-3" />
											</Button>
										</TableCell>
									</TableRow>
								);
							})
						)}
					</TableBody>
				</Table>
			</div>

			<Dialog open={mode.kind !== "closed"} onOpenChange={(open) => !open && resetDialog()}>
				<DialogContent className="max-w-2xl">
					<DialogHeader>
						<DialogTitle>
							{mode.kind === "edit" ? `Edit "${mode.group.name}"` : "New tool group"}
						</DialogTitle>
						<DialogDescription>
							Group tools together for discoverability. Example names: "read-only", "destructive", "customer-facing".
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-3">
						<div className="space-y-1">
							<Label htmlFor="mcp-tool-group-name">Name</Label>
							<Input
								id="mcp-tool-group-name"
								value={name}
								onChange={(e) => setName(e.target.value)}
							/>
						</div>
						<div className="space-y-1">
							<Label htmlFor="mcp-tool-group-desc">Description</Label>
							<Textarea
								id="mcp-tool-group-desc"
								rows={2}
								value={description}
								onChange={(e) => setDescription(e.target.value)}
							/>
						</div>
						<div className="space-y-2">
							<Label>Tools</Label>
							<div className="max-h-64 space-y-3 overflow-auto rounded-md border p-3">
								{clients.length === 0 ? (
									<p className="text-muted-foreground text-sm">No MCP clients configured.</p>
								) : (
									clients.map((c) => (
										<div key={c.config.client_id}>
											<div className="text-foreground mb-1 text-xs font-semibold uppercase tracking-wide">
												{c.config.name}
											</div>
											<div className="space-y-1 pl-2">
												{c.tools.length === 0 ? (
													<p className="text-muted-foreground text-xs">No discovered tools</p>
												) : (
													c.tools.map((t) => {
														const ref: MCPToolRef = {
															mcp_client_id: c.config.client_id,
															tool_name: t.name,
														};
														const checked = !!selected[refKey(ref)];
														return (
															<label key={t.name} className="flex items-center gap-2 text-sm">
																<Checkbox
																	checked={checked}
																	onCheckedChange={() => toggleRef(ref)}
																/>
																<span>{t.name}</span>
															</label>
														);
													})
												)}
											</div>
										</div>
									))
								)}
							</div>
							<p className="text-muted-foreground text-xs">
								{Object.keys(selected).length} tools selected
							</p>
						</div>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={resetDialog}>
							Cancel
						</Button>
						<Button onClick={handleSave} disabled={isCreating || isUpdating}>
							{isCreating || isUpdating ? "Saving…" : "Save"}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}
