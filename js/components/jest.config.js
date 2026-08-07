/** @type {import('jest').Config} */
module.exports = {
  preset: "jest-expo",
  // jest-expo's default transformIgnorePatterns plus the ESM-only packages this
  // package consumes: `streamplace` (workspace) and `@atproto/lex`.
  transformIgnorePatterns: [
    "node_modules/(?!((jest-)?react-native|@react-native(-community)?|expo(nent)?|@expo(nent)?/.*|@expo-google-fonts/.*|react-navigation|@react-navigation/.*|@sentry/react-native|native-base|react-native-svg|streamplace|@atproto/))",
  ],
};
