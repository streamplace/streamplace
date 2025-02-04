import { createAppSlice } from "../../hooks/createSlice";
import { initialState } from "./shared";

export const platformSlice = createAppSlice({
  name: "platform",
  initialState,
  reducers: (create) => ({
    openLoginLink: create.asyncThunk(
      async (url: string) => {
        window.location.href = url;
      },
      {
        pending: (state) => {
          state.status = "loading";
        },
        fulfilled: (state) => {
          state.status = "idle";
        },
        rejected: (state) => {
          state.status = "failed";
        },
      },
    ),

    initPushNotifications: create.asyncThunk(
      async () => {
        // someday we'll do web notifications but for now it's mobile-only
      },
      {
        pending: (state) => {},
        fulfilled: (state) => {},
        rejected: (state) => {},
      },
    ),

    registerNotificationToken: create.asyncThunk(async () => {}, {
      pending: (state) => {},
      fulfilled: (state) => {},
      rejected: (state) => {},
    }),
  }),

  selectors: {
    selectNotificationToken: (platform) => platform.notificationToken,
  },
});

export const { openLoginLink } = platformSlice.actions;
export const { selectNotificationToken } = platformSlice.selectors;
