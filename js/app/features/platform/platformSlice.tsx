import { createAppSlice } from "../../hooks/createSlice";

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
  }),
});

export const { openLoginLink } = platformSlice.actions;
