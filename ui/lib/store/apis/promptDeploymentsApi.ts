import type {
	PromptDeployment,
	PromptDeploymentLabel,
} from "@/lib/types/promptDeployments";
import { baseApi } from "./baseApi";

export const promptDeploymentsApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		getPromptDeployments: builder.query<{ deployments: PromptDeployment[] }, { promptId: string }>({
			query: ({ promptId }) => ({
				url: `/prompts/${encodeURIComponent(promptId)}/deployments`,
			}),
			providesTags: ["PromptDeployments"],
		}),
		upsertPromptDeployment: builder.mutation<
			PromptDeployment,
			{ promptId: string; label: PromptDeploymentLabel; version_id: number; promoted_by?: string }
		>({
			query: ({ promptId, label, version_id, promoted_by }) => ({
				url: `/prompts/${encodeURIComponent(promptId)}/deployments/${encodeURIComponent(label)}`,
				method: "PUT",
				body: { version_id, promoted_by },
			}),
			invalidatesTags: ["PromptDeployments"],
		}),
		deletePromptDeployment: builder.mutation<
			void,
			{ promptId: string; label: PromptDeploymentLabel }
		>({
			query: ({ promptId, label }) => ({
				url: `/prompts/${encodeURIComponent(promptId)}/deployments/${encodeURIComponent(label)}`,
				method: "DELETE",
			}),
			invalidatesTags: ["PromptDeployments"],
		}),
	}),
});

export const {
	useGetPromptDeploymentsQuery,
	useUpsertPromptDeploymentMutation,
	useDeletePromptDeploymentMutation,
} = promptDeploymentsApi;
