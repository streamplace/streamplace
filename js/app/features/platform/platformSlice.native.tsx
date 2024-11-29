import { openAuthSessionAsync } from "expo-web-browser";
import { createAppSlice } from "../../hooks/createSlice";
import { oauthCallback } from "features/bluesky/blueskySlice";

export interface PlatformState {
  status: "idle" | "loading" | "failed";
}

const initialState: PlatformState = {
  status: "idle",
};

export const platformSlice = createAppSlice({
  name: "platform",
  initialState,
  reducers: (create) => ({
    openLoginLink: create.asyncThunk(
      async (url: string, thunkAPI) => {
        console.log("openLoginLink", url);
        const res = await openAuthSessionAsync(url);
        if (res.type === "success") {
          thunkAPI.dispatch(oauthCallback(res.url));
        }
      },
      {
        pending: (state) => {
          state.status = "loading";
        },
        fulfilled: (state) => {
          state.status = "idle";
        },
        rejected: (state, { error }) => {
          state.status = "failed";
          console.error(error);
        },
      },
    ),
  }),
});

export const { openLoginLink } = platformSlice.actions;
