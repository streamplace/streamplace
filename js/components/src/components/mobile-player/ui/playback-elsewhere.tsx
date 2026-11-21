import { useTranslation } from "react-i18next";
import { Button, Text, usePlayerStore, useTheme, View, zero } from "../../..";

// Shown over a player whose owner session a newer one replaced; only the
// viewer's "Play here" takes playback back, so two players never fight.
export function PlaybackElsewhereOverlay() {
  const elsewhere = usePlayerStore((x) => x.playbackStartedElsewhere);
  return elsewhere ? <PlaybackElsewhere /> : null;
}

function PlaybackElsewhere() {
  const { t } = useTranslation("common");
  const { theme } = useTheme();
  const playHere = usePlayerStore((x) => x.playHere);
  return (
    <View
      testID="playback-elsewhere"
      style={[
        zero.layout.position.absolute,
        zero.position.top[0],
        zero.position.left[0],
        zero.position.right[0],
        zero.position.bottom[0],
        zero.layout.flex.center,
        zero.p[4],
        { backgroundColor: theme.colors.overlay },
      ]}
    >
      <View
        style={[
          zero.layout.flex.column,
          zero.layout.flex.alignCenter,
          zero.gap.all[4],
          zero.p[6],
          zero.r.lg,
          zero.borders.width.thin,
          {
            backgroundColor: theme.colors.surface1,
            borderColor: theme.colors.border,
          },
        ]}
      >
        <Text size="lg" weight="semibold" center>
          {t("playback-started-elsewhere")}
        </Text>
        <Button
          testID="playback-elsewhere-play-here"
          width="min"
          style={{ alignSelf: "center" }}
          onPress={() => playHere?.()}
        >
          {t("play-here")}
        </Button>
      </View>
    </View>
  );
}
