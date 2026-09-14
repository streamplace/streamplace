import { Button, Text, useBrandingAsset } from "@streamplace/components";

const BLUESKY_BLUE = "#2c68f6"; // token-ok: the OAuth (Bluesky / AT Protocol) sign-in button

/** The OAuth sign-in button: any AT Protocol account, Bluesky's included,
 *  through the node's own OAuth flow rather than the network's PDS login.
 *  Bluesky-blue so it reads as "the other network" beside the branded one. */
export function OAuthLoginButton({ onPress }: { onPress: () => void }) {
  const label =
    useBrandingAsset("signInOAuthLabel")?.data?.trim() ||
    "Sign in with Bluesky or AT Protocol";
  return (
    <Button
      variant="accent"
      width="full"
      onPress={onPress}
      style={{ backgroundColor: BLUESKY_BLUE, height: 44 }}
    >
      <Text
        weight="semibold"
        style={{ color: "#fff", fontSize: 15, lineHeight: 20 }} // token-ok: on the blue fill
      >
        {label}
      </Text>
    </Button>
  );
}
