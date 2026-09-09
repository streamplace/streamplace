import { CommonActions, useNavigation } from "@react-navigation/native";
import { Text, useBrandingAsset, useTheme } from "@streamplace/components";
import { Linking, Pressable, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { getStreamplaceStateFromPath } from "src/linking-config";
import { useStore } from "store";
import {
  BrandedNavLink,
  parseNavLinks,
  resolveNavUrl,
} from "./sidebar-overlay";
import { SOCIAL_NAV_ICONS } from "./social-icons";

// The timeline app's bottom bar order; whichever of these the node's
// navigation links carry, up to five, else the first five links.
const PREFERRED = ["home", "search", "live", "play", "bell", "message", "user"];
const MAX_ITEMS = 5;

export function pickTabBarLinks(links: BrandedNavLink[]): BrandedNavLink[] {
  const withIcon = links.filter((l) => l.icon && SOCIAL_NAV_ICONS[l.icon]);
  const preferred = PREFERRED.map((icon) =>
    withIcon.find((l) => l.icon === icon),
  ).filter((l): l is BrandedNavLink => !!l);
  const chosen = preferred.length >= 3 ? preferred : withIcon;
  return chosen.slice(0, MAX_ITEMS);
}

const isExternal = (url: string) => /^[a-z]+:/i.test(url);

/**
 * Mobile navigation for the social shell: the branded links as an icon bar
 * pinned to the bottom, the way the timeline app it imitates does.
 */
export function SocialTabBar() {
  const { theme } = useTheme();
  const insets = useSafeAreaInsets();
  const navigation = useNavigation();
  const closeDrawer = useStore((state) => state.closeDrawer);
  const onStreamPage = useStore((state) => state.wideColumn);
  const did = useStore((state) => state.oauthSession?.did);
  const viewerHandle = useStore((state) =>
    did ? state.profiles[did]?.handle : undefined,
  );
  const viewer = { did, handle: viewerHandle };
  const links = pickTabBarLinks(
    parseNavLinks(useBrandingAsset("navLinks")?.data),
  );
  if (links.length === 0) return null;

  const open = (raw: string) => {
    const url = resolveNavUrl(raw, viewer);
    if (isExternal(url)) {
      void Linking.openURL(url);
      return;
    }
    closeDrawer();
    const state = getStreamplaceStateFromPath(url);
    navigation.dispatch(
      CommonActions.reset({ index: 0, routes: state.routes }),
    );
  };

  return (
    <View
      style={{
        flexDirection: "row",
        alignItems: "stretch",
        height: 56 + insets.bottom,
        paddingBottom: insets.bottom,
        borderTopWidth: 1,
        borderTopColor: theme.colors.borderSubtle,
        backgroundColor: theme.colors.background,
      }}
    >
      {links.map((link) => {
        const icons = SOCIAL_NAV_ICONS[link.icon ?? ""];
        const active = link.url === "/" && onStreamPage;
        const Icon = active ? icons.active : icons.inactive;
        return (
          <Pressable
            key={link.url + link.label}
            accessibilityRole="tab"
            accessibilityLabel={link.label}
            accessibilityState={{ selected: active }}
            onPress={() => open(link.url)}
            style={{ flex: 1, alignItems: "center", justifyContent: "center" }}
          >
            <Icon size={26} color={theme.colors.text1} />
            <Text
              style={{
                fontSize: 10,
                lineHeight: 12,
                marginTop: 2,
                color: theme.colors.text1,
              }}
              numberOfLines={1}
            >
              {link.label}
            </Text>
          </Pressable>
        );
      })}
    </View>
  );
}
