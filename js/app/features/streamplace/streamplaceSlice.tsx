import { createAppSlice } from "../../hooks/createSlice";
import { isWeb } from "tamagui";
import { SignTypedDataFn } from "hooks/useWallet.shared";
import schema from "generated/eip712-schema.json";
import Storage from "../../storage";

let DEFAULT_URL = process.env.EXPO_PUBLIC_STREAMPLACE_URL as string;
if (isWeb && process.env.EXPO_PUBLIC_WEB_TRY_LOCAL === "true") {
  try {
    DEFAULT_URL = `${window.location.protocol}//${window.location.host}`;
  } catch (err) {
    // Oh well, fall back to hardcoded.
  }
}
export { DEFAULT_URL };

export interface Identity {
  id: string;
  handle?: string;
  did?: string;
}

export interface StreamplaceState {
  url: string;
  identity: Identity | null;
  initialized: boolean;
}

const initialState: StreamplaceState = {
  url: DEFAULT_URL,
  identity: null,
  initialized: false,
};

export const streamplaceSlice = createAppSlice({
  name: "streamplace",
  initialState,
  reducers: (create) => ({
    initialize: create.asyncThunk(
      async (_, { getState }) => {
        let url = await Storage.getItem("streamplaceUrl");
        if (!url) {
          url = DEFAULT_URL;
        }
        return url;
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          const url = action.payload;
          return {
            ...state,
            url,
            initialized: true,
          };
        },
        rejected: (_, { error }) => {
          // state.status = "failed";
        },
      },
    ),

    setURL: create.reducer((state, action: { payload: string }) => {
      Storage.setItem("streamplaceUrl", action.payload).catch((err) => {
        console.error("setURL error", err);
      });
      return {
        ...state,
        url: action.payload,
      };
    }),

    getIdentity: create.asyncThunk(
      async (_, { getState }) => {
        const { streamplace } = getState() as {
          streamplace: StreamplaceState;
        };
        const res = await fetch(`${streamplace.url}/api/identity`);
        return await res.json();
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          return {
            ...state,
            identity: action.payload,
          };
        },
        rejected: (state) => {
          console.error("loadOAuthClient rejected");
          // state.status = "failed";
        },
      },
    ),

    putIdentity: create.asyncThunk(
      async (
        {
          handle,
          did,
          address,
          signTypedData,
        }: {
          handle: string;
          did: string;
          address: string;
          signTypedData: SignTypedDataFn;
        },
        { getState, dispatch },
      ) => {
        let { streamplace } = getState() as {
          streamplace: StreamplaceState;
        };
        if (!streamplace.identity) {
          await dispatch(getIdentity());
        }
        ({ streamplace } = getState() as {
          streamplace: StreamplaceState;
        });
        if (!streamplace.identity) {
          throw new Error("No identity");
        }
        const message = {
          signer: address,
          time: Date.now(),
          data: { handle, did },
        };
        const toSign = {
          types: schema.types,
          domain: schema.domain as any,
          primaryType: "Identity",
          message: message,
        };
        const signature = await signTypedData(toSign);
        const res = await fetch(
          `${streamplace.url}/api/identity/${streamplace.identity.id}`,
          {
            method: "PUT",
            body: JSON.stringify({
              primaryType: "Identity",
              domain: schema.domain,
              message: message,
              signature: signature,
            }),
          },
        );
        if (!res.ok) {
          const text = await res.text();
          throw new Error(`http ${res.status} ${text}`);
        }

        return await res.json();
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {},
        rejected: (state, err) => {
          console.error("putIdentity rejected", err);
          // state.status = "failed";
        },
      },
    ),
  }),

  selectors: {
    selectStreamplace: (streamplace) => streamplace,
  },
});

// Action creators are generated for each case reducer function.
export const { getIdentity, putIdentity, setURL, initialize } =
  streamplaceSlice.actions;
export const { selectStreamplace } = streamplaceSlice.selectors;
