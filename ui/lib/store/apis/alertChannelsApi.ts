import type {
	AlertChannel,
	AlertChannelCreatePayload,
	AlertChannelUpdatePayload,
} from "@/lib/types/alertChannels";
import { baseApi } from "./baseApi";

// RTK query surface for /api/alert-channels (spec 004).
//
// All mutations invalidate the "AlertChannels" tag so the list auto-refetches.

export const alertChannelsApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		getAlertChannels: builder.query<{ channels: AlertChannel[] }, void>({
			query: () => ({ url: "/alert-channels" }),
			providesTags: ["AlertChannels"],
		}),
		createAlertChannel: builder.mutation<AlertChannel, AlertChannelCreatePayload>({
			query: (body) => ({
				url: "/alert-channels",
				method: "POST",
				body,
			}),
			invalidatesTags: ["AlertChannels"],
		}),
		updateAlertChannel: builder.mutation<AlertChannel, { id: string; patch: AlertChannelUpdatePayload }>({
			query: ({ id, patch }) => ({
				url: `/alert-channels/${encodeURIComponent(id)}`,
				method: "PATCH",
				body: patch,
			}),
			invalidatesTags: ["AlertChannels"],
		}),
		deleteAlertChannel: builder.mutation<void, { id: string }>({
			query: ({ id }) => ({
				url: `/alert-channels/${encodeURIComponent(id)}`,
				method: "DELETE",
			}),
			invalidatesTags: ["AlertChannels"],
		}),
		testAlertChannel: builder.mutation<{ dispatched: boolean }, { id: string }>({
			query: ({ id }) => ({
				url: `/alert-channels/${encodeURIComponent(id)}/test`,
				method: "POST",
			}),
		}),
	}),
});

export const {
	useGetAlertChannelsQuery,
	useCreateAlertChannelMutation,
	useUpdateAlertChannelMutation,
	useDeleteAlertChannelMutation,
	useTestAlertChannelMutation,
} = alertChannelsApi;
