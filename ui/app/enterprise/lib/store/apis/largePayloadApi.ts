// Real enterprise API for /api/config/large-payload (spec 006).
// Replaces the OSS-fallback stub that returned no-op hooks.

import type { LargePayloadConfig } from "../../types/largePayload";
import { baseApi } from "@/lib/store/apis/baseApi";

export const largePayloadApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		getLargePayloadConfig: builder.query<LargePayloadConfig, void>({
			query: () => ({ url: "/config/large-payload" }),
			providesTags: ["LargePayloadConfig"],
		}),
		updateLargePayloadConfig: builder.mutation<LargePayloadConfig, LargePayloadConfig>({
			query: (body) => ({
				url: "/config/large-payload",
				method: "PUT",
				body,
			}),
			invalidatesTags: ["LargePayloadConfig"],
		}),
	}),
});

export const { useGetLargePayloadConfigQuery, useUpdateLargePayloadConfigMutation } = largePayloadApi;
