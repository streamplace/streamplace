import {
  BioViewer,
  getPDSServiceEndpoint,
  resolveDIDDocument,
  Text,
  useAvatars,
  useLivestreamInfo,
  useTheme,
  View,
  zero,
} from "@streamplace/components";
import { Image } from "expo-image";
import { ChevronDown, ChevronUp } from "lucide-react-native";
import { useEffect, useState } from "react";
import { Pressable } from "react-native";
import type { PlaceStreamBioPage } from "streamplace";

const BIO_COLLECTION = "place.stream.bio.page";
const BIO_RKEY = "self";

function useStreamerBio(did: string | undefined) {
  const [bio, setBio] = useState<PlaceStreamBioPage.Record | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!did) return;
    let cancelled = false;
    setLoading(true);
    setBio(null);

    (async () => {
      try {
        const didDoc = await resolveDIDDocument(did);
        const pdsEndpoint = getPDSServiceEndpoint(didDoc);
        const params = new URLSearchParams({
          repo: did,
          collection: BIO_COLLECTION,
          rkey: BIO_RKEY,
        });
        const res = await fetch(
          `${pdsEndpoint}/xrpc/com.atproto.repo.getRecord?${params}`,
        );
        if (!res.ok) return;
        const json = await res.json();
        if (!cancelled) setBio(json.value as PlaceStreamBioPage.Record);
      } catch {
        // streamer has no bio or PDS is unreachable — render nothing
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [did]);

  return { bio, loading };
}

type Tab = "about";

const TABS: { id: Tab; label: string }[] = [{ id: "about", label: "About" }];

export function StreamTabs({
  onToggleCollapse,
  collapsed,
}: {
  onToggleCollapse?: () => void;
  collapsed?: boolean;
}) {
  const { profile } = useLivestreamInfo();
  const { bio, loading } = useStreamerBio(profile?.did);
  const [activeTab, setActiveTab] = useState<Tab>("about");
  const { theme } = useTheme();
  const avatars = useAvatars(profile?.did ? [profile.did] : []);
  const avatar = profile?.did ? avatars[profile.did]?.avatar : undefined;

  if (!profile?.did) return null;

  return (
    <View style={{ backgroundColor: "rgba(0,0,0,0.9)" }}>
      {/* Profile strip — tap to collapse/expand video */}
      {onToggleCollapse && (
        <Pressable
          onPress={onToggleCollapse}
          style={{
            flexDirection: "row",
            alignItems: "center",
            paddingHorizontal: 20,
            paddingVertical: 12,
            gap: 10,
          }}
        >
          <Image
            source={
              avatar ? { uri: avatar } : require("assets/images/goose.png")
            }
            style={{ width: 32, height: 32, borderRadius: 999 }}
          />
          <Text size="base" weight="medium" style={{ flex: 1 }}>
            {profile.handle}
          </Text>
          {collapsed ? (
            <ChevronDown size={18} color={theme.colors.mutedForeground} />
          ) : (
            <ChevronUp size={18} color={theme.colors.mutedForeground} />
          )}
        </Pressable>
      )}

      {/* Tab bar */}
      <View direction="row" style={{ paddingHorizontal: 20, paddingTop: 16 }}>
        {TABS.map((tab) => {
          const active = activeTab === tab.id;
          return (
            <Pressable
              key={tab.id}
              onPress={() => setActiveTab(tab.id)}
              style={{ marginRight: 24, paddingBottom: 12 }}
            >
              <Text
                size="xl"
                weight={active ? "bold" : "normal"}
                color={active ? undefined : "muted"}
              >
                {tab.label}
              </Text>
            </Pressable>
          );
        })}
      </View>

      {/* Tab content */}
      {activeTab === "about" && (
        <>
          {loading ? (
            <View style={[zero.p[4]]}>
              <Text color="muted" size="sm">
                Loading...
              </Text>
            </View>
          ) : bio ? (
            <BioViewer bio={bio} did={profile.did} />
          ) : (
            <View style={[zero.p[4]]}>
              <Text color="muted" size="sm">
                No bio yet.
              </Text>
            </View>
          )}
        </>
      )}
    </View>
  );
}
