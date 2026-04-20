import type {
	GuardrailProvider,
	GuardrailProviderCreatePayload,
	GuardrailProviderUpdatePayload,
	GuardrailRule,
	GuardrailRuleCreatePayload,
	GuardrailRuleUpdatePayload,
} from "@/lib/types/guardrails";
import { baseApi } from "./baseApi";

export const guardrailsApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		// Providers
		getGuardrailProviders: builder.query<{ providers: GuardrailProvider[] }, void>({
			query: () => ({ url: "/guardrails/providers" }),
			providesTags: ["Guardrails"],
		}),
		createGuardrailProvider: builder.mutation<GuardrailProvider, GuardrailProviderCreatePayload>({
			query: (body) => ({ url: "/guardrails/providers", method: "POST", body }),
			invalidatesTags: ["Guardrails"],
		}),
		updateGuardrailProvider: builder.mutation<
			GuardrailProvider,
			{ id: string; patch: GuardrailProviderUpdatePayload }
		>({
			query: ({ id, patch }) => ({
				url: `/guardrails/providers/${encodeURIComponent(id)}`,
				method: "PATCH",
				body: patch,
			}),
			invalidatesTags: ["Guardrails"],
		}),
		deleteGuardrailProvider: builder.mutation<void, { id: string }>({
			query: ({ id }) => ({
				url: `/guardrails/providers/${encodeURIComponent(id)}`,
				method: "DELETE",
			}),
			invalidatesTags: ["Guardrails", "GuardrailRules"],
		}),
		// Rules
		getGuardrailRules: builder.query<{ rules: GuardrailRule[] }, void>({
			query: () => ({ url: "/guardrails/rules" }),
			providesTags: ["GuardrailRules"],
		}),
		createGuardrailRule: builder.mutation<GuardrailRule, GuardrailRuleCreatePayload>({
			query: (body) => ({ url: "/guardrails/rules", method: "POST", body }),
			invalidatesTags: ["GuardrailRules"],
		}),
		updateGuardrailRule: builder.mutation<
			GuardrailRule,
			{ id: string; patch: GuardrailRuleUpdatePayload }
		>({
			query: ({ id, patch }) => ({
				url: `/guardrails/rules/${encodeURIComponent(id)}`,
				method: "PATCH",
				body: patch,
			}),
			invalidatesTags: ["GuardrailRules"],
		}),
		deleteGuardrailRule: builder.mutation<void, { id: string }>({
			query: ({ id }) => ({
				url: `/guardrails/rules/${encodeURIComponent(id)}`,
				method: "DELETE",
			}),
			invalidatesTags: ["GuardrailRules"],
		}),
	}),
});

export const {
	useGetGuardrailProvidersQuery,
	useCreateGuardrailProviderMutation,
	useUpdateGuardrailProviderMutation,
	useDeleteGuardrailProviderMutation,
	useGetGuardrailRulesQuery,
	useCreateGuardrailRuleMutation,
	useUpdateGuardrailRuleMutation,
	useDeleteGuardrailRuleMutation,
} = guardrailsApi;
