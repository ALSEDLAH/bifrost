import type { AccessControlReport, AdminActivityReport } from "@/lib/types/reports";
import { baseApi } from "./baseApi";

export const reportsApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		getAdminActivity: builder.query<AdminActivityReport, { days: number }>({
			query: ({ days }) => ({ url: `/reports/admin-activity`, params: { days } }),
		}),
		getAccessControl: builder.query<AccessControlReport, { days: number }>({
			query: ({ days }) => ({ url: `/reports/access-control`, params: { days } }),
		}),
	}),
});

export const { useGetAdminActivityQuery, useGetAccessControlQuery } = reportsApi;
