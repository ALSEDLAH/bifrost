import type { AdaptiveRoutingStatus } from "@/lib/types/adaptiveRouting";
import { baseApi } from "./baseApi";

export const adaptiveRoutingApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		getAdaptiveRoutingStatus: builder.query<AdaptiveRoutingStatus, void>({
			query: () => ({ url: "/adaptive-routing/status" }),
		}),
	}),
});

export const { useGetAdaptiveRoutingStatusQuery } = adaptiveRoutingApi;
