import { useRoute } from "@react-navigation/native";
import { useTheme, zero } from "@streamplace/components";
import { SmokesignalEventForm } from "components/live-dashboard/smokesignal-event-form";
import Loading from "components/loading/loading";
import { useEffect } from "react";
import { ScrollView, View } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";

const { p, layout, flex } = zero;

type SmokesignalEventParams = {
  defaultTitle?: string;
  streamUrl: string;
};

export default function SmokesignalEvent() {
  const { zero: z } = useTheme();
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const route = useRoute();
  const params = route.params as SmokesignalEventParams | undefined;

  useEffect(() => {
    if (isReady && !userProfile) {
      openLoginModal({ name: route.name, params: route.params });
    }
  }, [isReady, userProfile, openLoginModal, route.name, route.params]);

  if (!isReady) {
    return <Loading />;
  }

  if (!userProfile) {
    return <Loading />;
  }

  return (
    <ScrollView
      contentContainerStyle={[flex.values[1], p[4]]}
      style={[z.bg.background]}
    >
      <View style={[layout.flex.column, { maxWidth: 600, alignSelf: "center", width: "100%" }]}>
        <SmokesignalEventForm
          defaultTitle={params?.defaultTitle}
          streamUrl={params?.streamUrl ?? ""}
        />
      </View>
    </ScrollView>
  );
}
