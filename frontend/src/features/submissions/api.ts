// Browser-side endpoints. Transport (credentials, JSON, ApiError) lives in
// shared/api/client.ts.
import { apiRequest } from "@/shared/api/client";

export const submissionsApi = {
  deleteSubmission: (id: string): Promise<void> =>
    apiRequest<void>(`/api/submissions/${encodeURIComponent(id)}`, {
      method: "DELETE",
      allowNotFound: true,
    }),
};
