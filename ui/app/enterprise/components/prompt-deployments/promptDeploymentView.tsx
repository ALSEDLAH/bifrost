// Prompt deployment label manager (spec 011 phase 1).
//
// For the currently-selected prompt (from PromptContext), shows the
// production + staging labels with "move" and "clear" actions, plus
// the version history with "Promote to production/staging" buttons.
//
// Runtime resolution (inference calls resolving a promptName to the
// production label) is phase 2.

import { usePromptContext } from "@/components/prompts/context";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
	useDeletePromptDeploymentMutation,
	useGetPromptDeploymentsQuery,
	useGetVersionsQuery,
	useUpsertPromptDeploymentMutation,
} from "@/lib/store/apis";
import type { PromptDeployment, PromptDeploymentLabel } from "@/lib/types/promptDeployments";
import { Info, X } from "lucide-react";
import { useMemo } from "react";
import { toast } from "sonner";

const LABELS: PromptDeploymentLabel[] = ["production", "staging"];

export default function PromptDeploymentView({ omitTitle }: { omitTitle?: boolean } = {}) {
	const { selectedPromptId } = usePromptContext();
	const { data: depData } = useGetPromptDeploymentsQuery(
		{ promptId: selectedPromptId! },
		{ skip: !selectedPromptId },
	);
	const { data: verData } = useGetVersionsQuery(selectedPromptId!, { skip: !selectedPromptId });
	const [upsert, { isLoading: isUpserting }] = useUpsertPromptDeploymentMutation();
	const [remove] = useDeletePromptDeploymentMutation();

	const deployments = useMemo(() => depData?.deployments ?? [], [depData]);
	const versions = useMemo(() => verData?.versions ?? [], [verData]);

	const byLabel = useMemo(() => {
		const m: Partial<Record<PromptDeploymentLabel, PromptDeployment>> = {};
		for (const d of deployments) m[d.label] = d;
		return m;
	}, [deployments]);

	if (!selectedPromptId) {
		return (
			<p className="text-muted-foreground p-3 text-sm">
				Select a prompt to manage its deployments.
			</p>
		);
	}

	async function promote(label: PromptDeploymentLabel, version_id: number) {
		try {
			await upsert({ promptId: selectedPromptId!, label, version_id }).unwrap();
			toast.success(`Promoted v${version_id} to ${label}`);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	async function clearLabel(label: PromptDeploymentLabel) {
		if (!confirm(`Clear the ${label} label?`)) return;
		try {
			await remove({ promptId: selectedPromptId!, label }).unwrap();
			toast.success(`Cleared ${label} label`);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	}

	function labelForVersion(v: number): PromptDeploymentLabel[] {
		return LABELS.filter((l) => byLabel[l]?.version_id === v);
	}

	return (
		<div className="space-y-3" data-testid="prompt-deployment-view">
			{!omitTitle && (
				<div>
					<h3 className="text-base font-semibold tracking-tight">Deployments</h3>
					<p className="text-muted-foreground text-xs">
						Label a version as <b>production</b> or <b>staging</b> so future
						inference resolution can pin to the labeled version instead of the
						always-latest one.
					</p>
				</div>
			)}

			<Alert variant="default" className="border-blue-20">
				<Info className="h-4 w-4 text-blue-600" />
				<AlertDescription className="text-xs">
					Phase 1 stores the label. Runtime resolution (<code>promptName</code>
					→ production version at inference time) ships in phase 2.
				</AlertDescription>
			</Alert>

			<div className="space-y-2">
				{LABELS.map((label) => {
					const d = byLabel[label];
					return (
						<div
							key={label}
							className="flex items-center justify-between rounded-md border px-3 py-2"
						>
							<div className="flex items-center gap-2">
								<Badge variant={label === "production" ? "default" : "outline"}>
									{label}
								</Badge>
								{d ? (
									<span className="text-muted-foreground text-xs">
										→ v
										{versions.find((v) => v.id === d.version_id)?.version_number ??
											d.version_id}{" "}
										(promoted {new Date(d.promoted_at).toLocaleString()})
									</span>
								) : (
									<span className="text-muted-foreground text-xs">not set</span>
								)}
							</div>
							{d && (
								<Button variant="ghost" size="sm" onClick={() => clearLabel(label)}>
									<X className="h-3 w-3" />
								</Button>
							)}
						</div>
					);
				})}
			</div>

			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Version</TableHead>
							<TableHead>Created</TableHead>
							<TableHead>Labels</TableHead>
							<TableHead className="text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{versions.length === 0 ? (
							<TableRow>
								<TableCell colSpan={4} className="text-muted-foreground py-6 text-center text-xs">
									No versions yet.
								</TableCell>
							</TableRow>
						) : (
							versions.map((v) => {
								const vLabels = labelForVersion(v.id);
								return (
									<TableRow key={v.id}>
										<TableCell className="font-mono text-sm">v{v.version_number}</TableCell>
										<TableCell className="text-muted-foreground text-xs">
											{new Date(v.created_at).toLocaleString()}
										</TableCell>
										<TableCell>
											<div className="flex gap-1">
												{v.is_latest && <Badge variant="secondary">latest</Badge>}
												{vLabels.map((l) => (
													<Badge key={l} variant="default">
														{l}
													</Badge>
												))}
											</div>
										</TableCell>
										<TableCell className="flex justify-end gap-1">
											{LABELS.map((label) =>
												byLabel[label]?.version_id === v.id ? null : (
													<Button
														key={label}
														size="sm"
														variant="outline"
														disabled={isUpserting}
														onClick={() => promote(label, v.id)}
														className="h-7 text-[11px]"
													>
														→ {label}
													</Button>
												),
											)}
										</TableCell>
									</TableRow>
								);
							})
						)}
					</TableBody>
				</Table>
			</div>
		</div>
	);
}
