export interface PlatformState {
  status: "idle" | "loading" | "failed";
  notificationToken: string | null;
}

export const initialState: PlatformState = {
  status: "idle",
  notificationToken: null,
};

export type RegisterNotificationTokenBody = {
  token: string;
  repoDID?: string;
};
