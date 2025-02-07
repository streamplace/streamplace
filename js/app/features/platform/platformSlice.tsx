import { createAppSlice } from "../../hooks/createSlice";
import { initialState } from "./shared";

export const platformSlice = createAppSlice({
  name: "platform",
  initialState,
  reducers: (create) => ({
    handleNotification: create.reducer(
      (
        state,
        action: { payload: { [key: string]: string | object } | undefined },
      ) => {
        return state;
      },
    ),
    clearNotification: create.reducer((state) => {
      return {
        ...state,
        notificationDestination: null,
      };
    }),
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
    selectNotificationDestination: (platform) =>
      platform.notificationDestination,
  },
});

export const { openLoginLink, clearNotification } = platformSlice.actions;
export const { selectNotificationToken, selectNotificationDestination } =
  platformSlice.selectors;
