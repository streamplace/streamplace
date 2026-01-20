import { useNavigation } from "@react-navigation/native";
import { useBrandingAsset } from "@streamplace/components";
import { useEffect } from "react";

export default function useTitle(user: string) {
  const navigation = useNavigation();
  const title = useBrandingAsset("siteTitle")?.data || "Streamplace";
  useEffect(() => {
    navigation.setOptions({
      title: `@${user} on ${title}`,
    });
  }, [user, navigation]);
}
