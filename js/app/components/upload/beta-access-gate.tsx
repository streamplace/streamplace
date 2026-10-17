import { useLinkTo } from "@react-navigation/native";
import {
  Button,
  Text,
  useRequestBetaAccess,
  useTheme,
  View,
  zero,
  type BetaStatus,
} from "@streamplace/components";
import { Clapperboard, Clock } from "lucide-react-native";
import { useState } from "react";

// Shown on the upload page when the logged-in account hasn't been granted VOD
// upload access. For "none" it offers a request button that publishes a
// place.stream.beta.request; for "requested" it confirms the request is on
// file and points users at the app, where the grant notification lands.
export default function BetaAccessGate({
  feature,
  status,
  onRequested,
}: {
  feature: string;
  status: Exclude<BetaStatus, "granted">;
  onRequested: () => void;
}) {
  const { theme } = useTheme();
  const linkTo = useLinkTo();
  const requestAccess = useRequestBetaAccess(feature);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleRequest = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await requestAccess();
      onRequested();
    } catch (e: any) {
      console.error("failed to request beta access", e);
      setError(e?.message ?? "Something went wrong. Please try again.");
    } finally {
      setSubmitting(false);
    }
  };

  const requested = status === "requested";
  const Icon = requested ? Clock : Clapperboard;

  return (
    <View
      style={[
        zero.flex.values[1],
        zero.layout.flex.align.center,
        zero.layout.flex.justify.center,
        zero.px[6],
        zero.py[12],
      ]}
    >
      <View
        style={{
          maxWidth: 440,
          width: "100%",
          alignItems: "center",
          gap: 16,
        }}
      >
        <View
          style={{
            width: 64,
            height: 64,
            borderRadius: 32,
            backgroundColor: theme.colors.muted,
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Icon size={30} color={theme.colors.primary} />
        </View>

        <Text size="2xl" weight="semibold" style={{ textAlign: "center" }}>
          {requested ? "Access requested!" : "Video uploads are in beta"}
        </Text>

        <Text
          style={{ textAlign: "center", color: theme.colors.textMuted }}
          leading="relaxed"
        >
          {requested ? (
            <>
              Download and log in to{" "}
              <Text
                onPress={() => linkTo("/download")}
                style={{
                  color: theme.colors.primary,
                  textDecorationLine: "underline",
                }}
              >
                the Streamplace app
              </Text>{" "}
              to get notified the moment that you get access.
            </>
          ) : (
            "Uploading videos to Streamplace is invite-only for the moment. Request access and we'll notify you when you're off the waitlist."
          )}
        </Text>

        {!requested && (
          <Button
            variant="primary"
            onPress={handleRequest}
            loading={submitting}
            loadingText="Requesting…"
            disabled={submitting}
            style={{ marginTop: 4 }}
          >
            Request access
          </Button>
        )}

        {error && (
          <Text size="sm" style={{ color: theme.colors.destructive }}>
            {error}
          </Text>
        )}
      </View>
    </View>
  );
}
