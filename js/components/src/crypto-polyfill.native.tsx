// awkward use of require()? you betcha. but import() with Metro causes it to try and
// resolve the package at compile-time even if not installed, so here we are.
let rnqc: any | null = null;
let expoCrypto: any | null = null;
try {
  rnqc = require("react-native-quick-crypto");
} catch (err) {}
try {
  expoCrypto = require("expo-crypto");
} catch (err) {}

if (!rnqc && !expoCrypto) {
  throw new Error(
    "Livestreaming requires one of react-native-quick-crypto or expo-crypto",
  );
} else if (!rnqc && expoCrypto) {
  // @atproto/crypto dependencies expect crypto.getRandomValues to be a function
  if (typeof globalThis.crypto === "undefined") {
    globalThis.crypto = {} as any;
  }
  if (typeof globalThis.crypto.getRandomValues === "undefined") {
    globalThis.crypto.getRandomValues = expoCrypto.getRandomValues;
  }
}
