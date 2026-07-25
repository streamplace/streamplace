import { zero } from "@streamplace/components";
import MultistreamStatus from "components/live-dashboard/multistream-status";
import Loading from "components/loading/loading";
import { useEffect } from "react";
import { View } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";

const { flex } = zero;

export default function PopoutMultistream() {
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const hideSidebar = useStore((x) => x.setSidebarHidden);
  const showSidebar = useStore((x) => x.setSidebarUnhidden);

  useEffect(() => {
    hideSidebar();
    return () => {
      showSidebar();
    };
  }, [showSidebar]);

  if (!isReady) return <Loading />;
  if (!userProfile) {
    openLoginModal({ name: "PopoutMultistream" });
    return <Loading />;
  }

  return (
    <View style={[flex.values[1]]}>
      <MultistreamStatus />
    </View>
  );
}
