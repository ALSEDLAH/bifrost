import type {
	LogExportConnector,
	LogExportConnectorCreatePayload,
	LogExportConnectorType,
	LogExportConnectorUpdatePayload,
} from "@/lib/types/logExportConnectors";
import { baseApi } from "./baseApi";

export const logExportConnectorsApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		getLogExportConnectors: builder.query<
			{ connectors: LogExportConnector[] },
			{ type?: LogExportConnectorType } | void
		>({
			query: (arg) => {
				const type = arg && "type" in arg ? arg.type : undefined;
				return {
					url: "/log-export/connectors",
					params: type ? { type } : undefined,
				};
			},
			providesTags: ["LogExportConnectors"],
		}),
		createLogExportConnector: builder.mutation<LogExportConnector, LogExportConnectorCreatePayload>({
			query: (body) => ({
				url: "/log-export/connectors",
				method: "POST",
				body,
			}),
			invalidatesTags: ["LogExportConnectors"],
		}),
		updateLogExportConnector: builder.mutation<
			LogExportConnector,
			{ id: string; patch: LogExportConnectorUpdatePayload }
		>({
			query: ({ id, patch }) => ({
				url: `/log-export/connectors/${encodeURIComponent(id)}`,
				method: "PATCH",
				body: patch,
			}),
			invalidatesTags: ["LogExportConnectors"],
		}),
		deleteLogExportConnector: builder.mutation<void, { id: string }>({
			query: ({ id }) => ({
				url: `/log-export/connectors/${encodeURIComponent(id)}`,
				method: "DELETE",
			}),
			invalidatesTags: ["LogExportConnectors"],
		}),
	}),
});

export const {
	useGetLogExportConnectorsQuery,
	useCreateLogExportConnectorMutation,
	useUpdateLogExportConnectorMutation,
	useDeleteLogExportConnectorMutation,
} = logExportConnectorsApi;
