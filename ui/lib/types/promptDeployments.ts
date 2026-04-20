// Types for /api/prompts/:prompt_id/deployments (spec 011).

export type PromptDeploymentLabel = "production" | "staging";

export interface PromptDeployment {
	prompt_id: string;
	label: PromptDeploymentLabel;
	version_id: number;
	promoted_by: string;
	promoted_at: string;
}
