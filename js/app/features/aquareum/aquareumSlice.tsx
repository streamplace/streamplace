import { createAppSlice } from "../../hooks/createSlice";
import { isWeb } from "tamagui";
import { SignTypedDataFn } from "hooks/useWallet.shared";
import schema from "generated/eip712-schema.json";

let DEFAULT_URL = process.env.EXPO_PUBLIC_AQUAREUM_URL as string;
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

export interface AquareumState {
  url: string;
  identity: Identity | null;
}

const initialState: AquareumState = {
  url: DEFAULT_URL,
  identity: null,
};

export const aquareumSlice = createAppSlice({
  name: "aquareum",
  initialState,
  reducers: (create) => ({
    setURL: create.reducer((state, action: { payload: string }) => {
      return {
        ...state,
        url: action.payload,
      };
    }),

    getIdentity: create.asyncThunk(
      async (_, { getState }) => {
        const { aquareum } = getState() as {
          aquareum: AquareumState;
        };
        const res = await fetch(`${aquareum.url}/api/identity`);
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
        let { aquareum } = getState() as {
          aquareum: AquareumState;
        };
        if (!aquareum.identity) {
          await dispatch(getIdentity());
        }
        ({ aquareum } = getState() as {
          aquareum: AquareumState;
        });
        if (!aquareum.identity) {
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
          `${aquareum.url}/api/identity/${aquareum.identity.id}`,
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
    selectAquareum: (aquareum) => aquareum,
  },
});

// Action creators are generated for each case reducer function.
export const { getIdentity, putIdentity, setURL } = aquareumSlice.actions;
export const { selectAquareum } = aquareumSlice.selectors;
