import type { ClusterStatus } from "@/lib/types/cluster";
import { baseApi } from "./baseApi";

export const clusterApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		getClusterStatus: builder.query<ClusterStatus, void>({
			query: () => ({ url: "/cluster/status" }),
			providesTags: ["ClusterNodes"],
		}),
	}),
});

export const { useGetClusterStatusQuery } = clusterApi;
