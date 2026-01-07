import { useNavigation } from "@react-navigation/native";
import { storage, Text, useTheme, zero } from "@streamplace/components";
import { Redirect } from "components/aqlink";
import Loading from "components/loading/loading";
import { useEffect, useState } from "react";
import { KeyboardAvoidingView, Platform, ScrollView, View } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";
import { navigateToRoute } from "../../utils/navigation";
import LoginForm from "./login-form";

export default function Login() {
  const { theme } = useTheme();
  const closeLoginModal = useStore((state) => state.closeLoginModal);
  const userProfile = useUserProfile();
  const navigation = useNavigation();
  const isReady = useIsReady();
  const [localReturnRoute, setLocalReturnRoute] = useState<
    | {
        name: string;
        params?: any;
      }
    | null
    | undefined
  >();

  // check for stored return route on mount
  useEffect(() => {
    storage.getItem("returnRoute").then((stored) => {
      if (stored) {
        try {
          const route = JSON.parse(stored);
          console.log("Login page - found stored returnRoute:", route);
          setLocalReturnRoute(route);
          storage.removeItem("returnRoute");
          closeLoginModal();
          navigateToRoute(navigation, route);
        } catch (e) {
          console.error("Failed to parse returnRoute from storage", e);
          setLocalReturnRoute(null);
        }
      } else {
        setLocalReturnRoute(null);
      }
    });
  }, [navigation, closeLoginModal]);

  if (!isReady || localReturnRoute === undefined) {
    return (
      <View
        style={[
          zero.flex.values[1],
          { justifyContent: "center", alignItems: "stretch" },
          zero.gap.all[3],
        ]}
      >
        <Loading />
      </View>
    );
  }

  if (userProfile) {
    // if return route is set, go there
    if (localReturnRoute) {
      <Redirect
        to={{ screen: localReturnRoute.name, params: localReturnRoute.params }}
      />;
    }
    return (
      <Redirect
        to={{ screen: "Settings", params: { screen: "AccountCategory" } }}
      />
    );
  }

  return (
    <KeyboardAvoidingView
      style={{ flex: 1 }}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <ScrollView contentContainerStyle={{ flexGrow: 1 }}>
        <View
          style={[
            zero.flex.values[1],
            { justifyContent: "center", alignItems: "center" },
            zero.p[4],
            { width: "100%" },
            { marginHorizontal: "auto" },
          ]}
        >
          <View
            style={[
              zero.px[8],
              zero.pt[10],
              zero.pb[6],
              zero.r.lg,
              { backgroundColor: theme.colors.card },
              { width: "100%" },
              { maxWidth: 600 },
              zero.gap.all[4],
            ]}
          >
            <Text style={[{ fontSize: 36, fontWeight: "200", color: "white" }]}>
              Log in
            </Text>
            <LoginForm />
          </View>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
