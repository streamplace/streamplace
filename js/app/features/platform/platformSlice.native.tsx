import { createAppSlice } from "../../hooks/createSlice";
import { openAuthSessionAsync } from "expo-web-browser";

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
      async (url: string) => {
        console.log("openLoginLink", url);
        const res = await openAuthSessionAsync(url);
        console.log(res);
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
