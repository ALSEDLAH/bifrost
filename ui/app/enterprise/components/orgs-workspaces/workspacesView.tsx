// Enterprise workspaces list view (US1).
//
// Renders a table of workspaces scoped to the caller's organization,
// plus a "New workspace" dialog. Wired to RTK Query via
// useGetWorkspacesQuery + useCreateWorkspaceMutation.
//
// data-testid convention: workspace-<element>-<qualifier> (see
// AGENTS.md "Gotchas & Conventions").

import { getErrorMessage } from "@/lib/store/apis/baseApi";
import {
	useCreateWorkspaceMutation,
	useDeleteWorkspaceMutation,
	useGetWorkspacesQuery,
} from "@/lib/store/apis/enterpriseApi";
import type { EnterpriseWorkspace } from "@/lib/types/enterprise";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { Link } from "@tanstack/react-router";
import { Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

export function WorkspacesView() {
	const { data: workspaces, isLoading, error } = useGetWorkspacesQuery();
	const [createOpen, setCreateOpen] = useState(false);

	return (
		<div className="flex w-full flex-col gap-6">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-semibold">Workspaces</h1>
					<p className="text-muted-foreground text-sm">
						Isolate virtual keys, prompts, configs, and logs per team.
					</p>
				</div>
				<Dialog open={createOpen} onOpenChange={setCreateOpen}>
					<DialogTrigger asChild>
						<Button data-testid="workspace-button-new">
							<Plus className="h-4 w-4" /> New workspace
						</Button>
					</DialogTrigger>
					<CreateWorkspaceDialogContent onClose={() => setCreateOpen(false)} />
				</Dialog>
			</div>

			{isLoading ? (
				<div className="text-muted-foreground text-sm" data-testid="workspace-state-loading">
					Loading…
				</div>
			) : error ? (
				<div className="text-destructive text-sm" data-testid="workspace-state-error">
					{getErrorMessage(error)}
				</div>
			) : !workspaces || workspaces.length === 0 ? (
				<EmptyState onCreate={() => setCreateOpen(true)} />
			) : (
				<WorkspacesTable rows={workspaces} />
			)}
		</div>
	);
}

function EmptyState({ onCreate }: { onCreate: () => void }) {
	return (
		<div
			className="border-muted flex flex-col items-center gap-4 rounded-lg border border-dashed p-12 text-center"
			data-testid="workspace-state-empty"
		>
			<p className="text-muted-foreground">No workspaces yet.</p>
			<Button onClick={onCreate} data-testid="workspace-button-add">
				<Plus className="h-4 w-4" /> Create first workspace
			</Button>
		</div>
	);
}

function WorkspacesTable({ rows }: { rows: EnterpriseWorkspace[] }) {
	const [deleteWorkspace] = useDeleteWorkspaceMutation();

	const handleDelete = async (id: string, name: string) => {
		const confirmed = window.confirm(
			`Delete workspace "${name}"? It will be soft-deleted with a 30-day grace period; its virtual keys are revoked immediately.`,
		);
		if (!confirmed) return;
		try {
			await deleteWorkspace(id).unwrap();
			toast.success(`Workspace "${name}" scheduled for deletion.`);
		} catch (err) {
			toast.error(`Failed to delete workspace: ${getErrorMessage(err)}`);
		}
	};

	return (
		<Table data-testid="workspace-list-table">
			<TableHeader>
				<TableRow>
					<TableHead>Name</TableHead>
					<TableHead>Slug</TableHead>
					<TableHead>Description</TableHead>
					<TableHead>Payload encryption</TableHead>
					<TableHead className="w-32">Actions</TableHead>
				</TableRow>
			</TableHeader>
			<TableBody>
				{rows.map((w) => (
					<TableRow key={w.id} data-testid={`workspace-list-row-${w.slug}`}>
						<TableCell className="font-medium">
							<Link to={`/workspace/workspaces/${w.id}` as any}>{w.name}</Link>
						</TableCell>
						<TableCell className="text-muted-foreground font-mono text-sm">{w.slug}</TableCell>
						<TableCell className="text-muted-foreground max-w-xs truncate">{w.description || "—"}</TableCell>
						<TableCell>{w.payload_encryption_enabled ? "Enabled" : "Off"}</TableCell>
						<TableCell>
							<Button
								variant="ghost"
								size="icon"
								onClick={() => handleDelete(w.id, w.name)}
								data-testid={`workspace-button-delete-${w.slug}`}
							>
								<Trash2 className="text-destructive h-4 w-4" />
							</Button>
						</TableCell>
					</TableRow>
				))}
			</TableBody>
		</Table>
	);
}

function CreateWorkspaceDialogContent({ onClose }: { onClose: () => void }) {
	const [name, setName] = useState("");
	const [slug, setSlug] = useState("");
	const [description, setDescription] = useState("");
	const [createWorkspace, { isLoading }] = useCreateWorkspaceMutation();

	const submit = async () => {
		try {
			await createWorkspace({ name, slug, description }).unwrap();
			toast.success(`Workspace "${name}" created.`);
			onClose();
			setName("");
			setSlug("");
			setDescription("");
		} catch (err) {
			toast.error(`Create failed: ${getErrorMessage(err)}`);
		}
	};

	return (
		<DialogContent data-testid="workspace-dialog-create">
			<DialogHeader>
				<DialogTitle>New workspace</DialogTitle>
				<DialogDescription>
					A workspace owns its own virtual keys, prompts, configs, and logs. Slug must be unique within this
					organization.
				</DialogDescription>
			</DialogHeader>
			<div className="grid gap-4 py-4">
				<div className="grid gap-2">
					<Label htmlFor="workspace-input-name">Name</Label>
					<Input
						id="workspace-input-name"
						data-testid="workspace-input-name"
						value={name}
						onChange={(e) => setName(e.target.value)}
						placeholder="Product Team"
					/>
				</div>
				<div className="grid gap-2">
					<Label htmlFor="workspace-input-slug">Slug</Label>
					<Input
						id="workspace-input-slug"
						data-testid="workspace-input-slug"
						value={slug}
						onChange={(e) => setSlug(e.target.value.toLowerCase())}
						placeholder="product"
					/>
					<p className="text-muted-foreground text-xs">Lowercase, URL-safe. e.g. "product", "data-science".</p>
				</div>
				<div className="grid gap-2">
					<Label htmlFor="workspace-input-description">Description</Label>
					<Textarea
						id="workspace-input-description"
						data-testid="workspace-input-description"
						value={description}
						onChange={(e) => setDescription(e.target.value)}
						rows={3}
					/>
				</div>
			</div>
			<DialogFooter>
				<Button variant="outline" onClick={onClose} data-testid="workspace-button-cancel">
					Cancel
				</Button>
				<Button
					onClick={submit}
					disabled={!name || !slug || isLoading}
					data-testid="workspace-button-submit"
				>
					{isLoading ? "Creating…" : "Create workspace"}
				</Button>
			</DialogFooter>
		</DialogContent>
	);
}
