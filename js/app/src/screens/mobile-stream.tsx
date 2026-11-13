import { useLinkTo, useNavigation } from "@react-navigation/native";
import {
  Button,
  KeepAwake,
  LivestreamProvider,
  PlayerProvider,
  Text,
  useTheme,
  View,
} from "@streamplace/components";
import { Player } from "components/mobile/player";
import { PlayerProps } from "components/player/props";
import { FullscreenProvider } from "contexts/FullscreenContext";
import useTitle from "hooks/useTitle";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Platform } from "react-native";
import { queryToProps } from "./util";

const isWeb = Platform.OS === "web";

function NotFoundPage() {
  const { t } = useTranslation("common");
  const { theme } = useTheme();
  const linkTo = useLinkTo();
  const navigation = useNavigation();
  const title = t("page-not-found-title");
  useEffect(() => {
    navigation.setOptions({ title });
  }, [navigation, title]);

  return (
    <View
      style={{
        flex: 1,
        alignItems: "center",
        justifyContent: "center",
        padding: theme.spacing[6],
        gap: theme.spacing[4],
        backgroundColor: theme.colors.background,
      }}
    >
      <Text role="heading" size="xl" weight="semibold" center>
        {t("page-not-found-title")}
      </Text>
      <Text color="muted" center>
        {t("page-not-found-description")}
      </Text>
      <Button onPress={() => linkTo("/")}>{t("nav-home")}</Button>
    </View>
  );
}

function MobileStreamInner({
  user,
  src,
  extraProps,
  onTeleport,
}: {
  user: string;
  src: string;
  extraProps: Partial<PlayerProps>;
  onTeleport?: (targetHandle: string, targetDID: string) => void;
}) {
  useTitle(user);

  return (
    <>
      <KeepAwake />
      <FullscreenProvider>
        <Player key={src} src={src} {...extraProps} onTeleport={onTeleport} />
      </FullscreenProvider>
    </>
  );
}

export default function MobileStream({ route }) {
  const { user } = route?.params ?? {};
  let navi = useNavigation();
  let extraProps: Partial<PlayerProps> = {};
  if (isWeb) {
    extraProps = queryToProps(new URLSearchParams(window.location.search));
  }
  const src = user;

  const handleTeleport = (targetHandle: string, targetDID?: string) => {
    if (!navi || (!targetHandle && !targetDID)) {
      console.error("Navigation or target info missing for teleport");
      return;
    }
    navi.navigate("Stream", {
      user: targetHandle,
    });
  };

  if (!/[.:]/.test(user ?? "")) return <NotFoundPage />;

  return (
    <LivestreamProvider key={src} src={src} onTeleport={handleTeleport}>
      <PlayerProvider>
        <MobileStreamInner
          user={user}
          src={src}
          extraProps={extraProps}
          onTeleport={handleTeleport}
        />
      </PlayerProvider>
    </LivestreamProvider>
  );
}
