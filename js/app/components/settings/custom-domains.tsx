import {
  Button,
  Input,
  MenuGroup,
  MenuInfo,
  MenuItem,
  MenuLabel,
  MenuSeparator,
  Text,
  useDID,
  useIsBrandAdmin,
  useStreamplaceStore,
  useToast,
  useTranslation,
  View,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { useCallback, useEffect, useState } from "react";
import { place } from "streamplace";
import { SettingsRowItem } from "./components/settings-navigation-item";

type DomainView = place.stream.branding.defs.DomainView;

// Whether the signed-in account owns a custom domain on this node: an owner
// who is not a node admin still manages their domain's brand from here.
export function useOwnsCustomDomain(): boolean {
  const agent = usePDSAgent();
  const did = useDID();
  const [owns, setOwns] = useState(false);
  useEffect(() => {
    if (!agent || !did) {
      setOwns(false);
      return;
    }
    let stopped = false;
    agent.client
      .call(place.stream.branding.listDomains)
      .then((res) => {
        if (!stopped) setOwns(res.domains.some((d) => d.owner === did));
      })
      .catch(() => {
        if (!stopped) setOwns(false);
      });
    return () => {
      stopped = true;
    };
  }, [agent, did]);
  return owns;
}

// Custom domains: hostnames the node serves under their owner's brand (the
// owner's place.stream.branding.brand record keyed by the hostname). Admins
// grant them; an owner edits a domain's brand by picking it here, which
// points the rest of the branding screen at it. Those edits are published
// to the owner's repo as the record.
export function CustomDomains({
  target,
  onEdit,
}: {
  // The brand the rest of the screen edits (a broadcaster ID).
  target: string;
  onEdit: (brand: string) => void;
}) {
  const { t } = useTranslation("settings");
  const agent = usePDSAgent();
  const toast = useToast();
  const did = useDID();
  const adminDIDs = useStreamplaceStore((s) => s.adminDIDs);
  const isAdmin = !!did && adminDIDs.includes(did);
  // May the caller edit the brand of the hostname the app is on?
  const isHostBrandAdmin = useIsBrandAdmin(did);
  const [domains, setDomains] = useState<DomainView[] | null>(null);
  const [hostname, setHostname] = useState("");
  const [owner, setOwner] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    if (!agent) return;
    try {
      const res = await agent.client.call(place.stream.branding.listDomains);
      setDomains(res.domains);
    } catch (e: any) {
      // A node without custom domains support answers 404: hide the section.
      setDomains([]);
    }
  }, [agent]);

  useEffect(() => {
    load();
  }, [load]);

  // An owner who cannot edit this hostname's own brand came here for their
  // domain: point the screen at it.
  useEffect(() => {
    if (isHostBrandAdmin || !domains || !did) return;
    const mine = domains.filter((d) => d.owner === did);
    if (mine.length > 0 && !mine.some((d) => d.brand === target)) {
      onEdit(mine[0].brand);
    }
  }, [domains, did, isHostBrandAdmin, target, onEdit]);

  const run = async (what: string, fn: () => Promise<unknown>) => {
    setBusy(true);
    try {
      await fn();
      await load();
    } catch (e: any) {
      toast.show(what, e?.message, { variant: "error" });
    } finally {
      setBusy(false);
    }
  };

  const add = () =>
    run(t("branding-domains-add-failed"), async () => {
      let ownerDID = owner.trim() || undefined;
      if (ownerDID && !ownerDID.startsWith("did:")) {
        const res = await agent!.com.atproto.identity.resolveHandle({
          handle: ownerDID.replace(/^@/, ""),
        });
        ownerDID = res.data.did;
      }
      await agent!.client.call(place.stream.branding.putDomain, {
        hostname: hostname.trim(),
        owner: ownerDID as any,
      });
      setHostname("");
      setOwner("");
    });

  if (!agent || domains === null || (!isAdmin && domains.length === 0)) {
    return null;
  }

  return (
    <>
      <MenuLabel>{t("branding-domains")}</MenuLabel>
      <MenuGroup>
        <MenuItem>
          <SettingsRowItem>
            <Text size="xs" color="muted">
              {t("branding-domains-description")}
            </Text>
          </SettingsRowItem>
        </MenuItem>
        {domains.map((d) => (
          <View key={d.hostname}>
            <MenuSeparator />
            <MenuItem>
              <SettingsRowItem>
                <View
                  style={[zero.gap.all[2], { flex: 1 }]}
                  testID={`branding-domain-${d.hostname}`}
                >
                  <Text size="sm" weight="semibold">
                    {d.hostname}
                  </Text>
                  <Text size="xs" color="muted">
                    {d.syncError
                      ? t("branding-domains-sync-error", { error: d.syncError })
                      : d.recordCid
                        ? t("branding-domains-synced", { owner: d.owner })
                        : t("branding-domains-no-record", { owner: d.owner })}
                  </Text>
                  <View
                    style={[
                      zero.layout.flex.direction.row,
                      zero.gap.all[2],
                      { flexWrap: "wrap" },
                    ]}
                  >
                    {d.owner === did && (
                      <Button
                        testID={`branding-domain-edit-${d.hostname}`}
                        onPress={() => onEdit(d.brand)}
                        disabled={busy || target === d.brand}
                        width="min"
                        style={{ height: 36 }}
                      >
                        {target === d.brand
                          ? t("branding-domains-editing")
                          : t("branding-domains-edit")}
                      </Button>
                    )}
                    <Button
                      variant="secondary"
                      onPress={() =>
                        run(t("branding-domains-sync-failed"), () =>
                          agent.client.call(place.stream.branding.syncDomain, {
                            hostname: d.hostname,
                          }),
                        )
                      }
                      disabled={busy}
                      width="min"
                      style={{ height: 36 }}
                    >
                      {t("branding-domains-sync")}
                    </Button>
                    <Button
                      variant="secondary"
                      onPress={() =>
                        run(t("branding-domains-remove-failed"), () =>
                          agent.client.call(
                            place.stream.branding.deleteDomain,
                            { hostname: d.hostname },
                          ),
                        )
                      }
                      disabled={busy}
                      width="min"
                      style={{ height: 36 }}
                    >
                      {t("branding-domains-remove")}
                    </Button>
                  </View>
                </View>
              </SettingsRowItem>
            </MenuItem>
          </View>
        ))}
        {isAdmin && (
          <>
            <MenuSeparator />
            <MenuItem>
              <SettingsRowItem>
                <View style={[zero.gap.all[2], { flex: 1 }]}>
                  <Text size="sm" weight="semibold">
                    {t("branding-domains-add")}
                  </Text>
                  <Input
                    testID="branding-domain-hostname"
                    placeholder="live.example.com"
                    value={hostname}
                    onChangeText={setHostname}
                    autoCapitalize="none"
                  />
                  <Input
                    testID="branding-domain-owner"
                    placeholder={t("branding-domains-owner-placeholder")}
                    value={owner}
                    onChangeText={setOwner}
                    autoCapitalize="none"
                  />
                  <Button
                    testID="branding-domain-add"
                    onPress={add}
                    disabled={busy || !hostname.trim()}
                    width="min"
                    style={{ height: 42 }}
                  >
                    {t("branding-domains-add-button")}
                  </Button>
                  <MenuInfo description={t("branding-domains-add-help")} />
                </View>
              </SettingsRowItem>
            </MenuItem>
          </>
        )}
      </MenuGroup>
    </>
  );
}
