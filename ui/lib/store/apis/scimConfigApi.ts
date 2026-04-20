import type { SCIMConfig, SCIMRotateResponse } from "@/lib/types/scim";
import { baseApi } from "./baseApi";

export const scimConfigApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		getSCIMConfig: builder.query<SCIMConfig, void>({
			query: () => ({ url: "/scim/config" }),
			providesTags: ["SCIMConfig"],
		}),
		patchSCIMConfig: builder.mutation<SCIMConfig, { enabled?: boolean }>({
			query: (patch) => ({
				url: "/scim/config",
				method: "PATCH",
				body: patch,
			}),
			invalidatesTags: ["SCIMConfig"],
		}),
		rotateSCIMToken: builder.mutation<SCIMRotateResponse, void>({
			query: () => ({
				url: "/scim/config/rotate",
				method: "POST",
			}),
			invalidatesTags: ["SCIMConfig"],
		}),
	}),
});

export const {
	useGetSCIMConfigQuery,
	usePatchSCIMConfigMutation,
	useRotateSCIMTokenMutation,
} = scimConfigApi;
