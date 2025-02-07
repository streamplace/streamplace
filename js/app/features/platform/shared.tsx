export interface PlatformState {
  status: "idle" | "loading" | "failed";
  notificationToken: string | null;
  notificationDestination: string | null;
}

export const initialState: PlatformState = {
  status: "idle",
  notificationToken: null,
  notificationDestination: null,
};

export type RegisterNotificationTokenBody = {
  token: string;
  repoDID?: string;
};
