import { useLinkTo, useNavigation } from "@react-navigation/native";
import {
  Button,
  KeepAwake,
  LivestreamProvider,
  PlayerProvider,
  Text,
  useTheme,
  useUrl,
  View,
} from "@streamplace/components";
import { useActorLookup } from "@streamplace/core";
import Loading from "components/loading/loading";
import { Player } from "components/mobile/player";
import { PlayerProps } from "components/player/props";
import { FullscreenProvider } from "contexts/FullscreenContext";
import useTitle from "hooks/useTitle";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Platform } from "react-native";
import { queryToProps } from "./util";

const isWeb = Platform.OS === "web";

function StreamRouteState({
  title,
  description,
  retry,
}: {
  title: string;
  description: string;
  retry?: () => void;
}) {
  const { t } = useTranslation("common");
  const { theme } = useTheme();
  const linkTo = useLinkTo();
  const navigation = useNavigation();
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
        {title}
      </Text>
      <Text color="muted" center>
        {description}
      </Text>
      <View style={{ flexDirection: "row", gap: theme.spacing[2] }}>
        {retry && (
          <Button width="min" onPress={retry}>
            {t("try-again")}
          </Button>
        )}
        <Button width="min" onPress={() => linkTo("/")}>
          {t("nav-home")}
        </Button>
      </View>
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
  const { t } = useTranslation("common");
  if (!/[.:]/.test(user ?? "")) {
    return (
      <StreamRouteState
        title={t("page-not-found-title")}
        description={t("page-not-found-description")}
      />
    );
  }
  return <AccountStream key={user} user={user} />;
}

function AccountStream({ user }: { user: string }) {
  const { t } = useTranslation("common");
  const nodeUrl = useUrl();
  const actor = useActorLookup(user, nodeUrl);
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

  if (actor.status === "not-found") {
    return (
      <StreamRouteState
        title={t("creator-not-found-title")}
        description={t("creator-not-found-description", {
          handle: user.startsWith("did:") ? user : `@${user}`,
        })}
      />
    );
  }
  if (actor.status === "error") {
    return (
      <StreamRouteState
        title={t("creator-lookup-failed-title")}
        description={t("creator-lookup-failed-description")}
        retry={actor.retry}
      />
    );
  }
  if (actor.status === "loading") return <Loading />;

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
