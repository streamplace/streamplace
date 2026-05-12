import {
  BioViewer,
  getPDSServiceEndpoint,
  resolveDIDDocument,
  Text,
  useLivestreamInfo,
  useTheme,
  View,
  zero,
} from "@streamplace/components";
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

export function StreamTabs() {
  const { profile } = useLivestreamInfo();
  const { bio, loading } = useStreamerBio(profile?.did);
  const [activeTab, setActiveTab] = useState<Tab>("about");
  const { theme } = useTheme();

  if (!profile?.did) return null;

  return (
    <View style={{ backgroundColor: "rgba(0,0,0,0.9)" }}>
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
