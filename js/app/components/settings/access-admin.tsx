import {
  Button,
  Input,
  MenuContainer,
  MenuGroup,
  MenuItem,
  MenuLabel,
  MenuSeparator,
  SegmentedTabs,
  Text,
  useAccessStatus,
  useAvatars,
  useDID,
  useFetchAccessStatus,
  useIsAdmin,
  useTheme,
  useToast,
  useTranslation,
  View,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { Lock } from "lucide-react-native";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ActivityIndicator, ScrollView } from "react-native";
import { place } from "streamplace";
import { SettingsRowItem } from "./components/settings-navigation-item";

type GrantView = place.stream.access.defs.GrantView;

// Every role an admin can configure. `admin` itself is always allowlist-only
// and is rejected by updatePolicy, so it is not offered here.
const POLICY_ROLES = ["viewer", "streamer", "syndicate", "vod"] as const;
const GRANT_ROLES = ["admin", ...POLICY_ROLES] as const;
const MODES = ["open", "allowlist", "off"] as const;

export function AccessAdmin() {
  const { t } = useTranslation("settings");
  const { theme } = useTheme();
  const agent = usePDSAgent();
  const did = useDID();
  const isAdmin = useIsAdmin();
  const accessStatus = useAccessStatus();
  const fetchAccessStatus = useFetchAccessStatus();
  const toast = useToast();

  const [busy, setBusy] = useState(false);
  const [grants, setGrants] = useState<GrantView[] | null>(null);
  const [grantsLoading, setGrantsLoading] = useState(false);
  const [roleFilter, setRoleFilter] = useState<string>("all");
  const [subject, setSubject] = useState("");
  const [note, setNote] = useState("");
  const [newRole, setNewRole] = useState<string>("viewer");
  const [pendingDelete, setPendingDelete] = useState<string | null>(null);

  const modeOptions = useMemo(
    () => MODES.map((m) => ({ label: t(`access-mode-${m}`), value: m })),
    [t],
  );
  const grantRoleOptions = useMemo(
    () => GRANT_ROLES.map((r) => ({ label: t(`access-role-${r}`), value: r })),
    [t],
  );
  const filterOptions = useMemo(
    () => [
      { label: t("access-filter-all"), value: "all" },
      ...grantRoleOptions,
    ],
    [t, grantRoleOptions],
  );

  const canManage = !!agent && !!did && isAdmin;

  const loadGrants = useCallback(async () => {
    if (!canManage || !agent) return;
    setGrantsLoading(true);
    try {
      const res = await agent.client.call(place.stream.access.listGrants, {});
      setGrants(res.grants);
    } catch (e: any) {
      console.error("failed to list grants", e);
      toast.show(
        t("access-grants-load-failed"),
        e?.message ?? t("access-grants-load-failed"),
        { variant: "error" },
      );
    } finally {
      setGrantsLoading(false);
    }
  }, [agent, canManage, t, toast]);

  useEffect(() => {
    loadGrants();
  }, [loadGrants]);

  // Resolve grant subjects to handles where the public AppView knows them.
  const subjectDids = useMemo(
    () => Array.from(new Set((grants ?? []).map((g) => g.subject as string))),
    [grants],
  );
  const profiles = useAvatars(subjectDids);

  const setMode = async (role: string, mode: string) => {
    if (!agent || accessStatus?.policy[role] === mode) return;
    setBusy(true);
    try {
      await agent.client.call(place.stream.access.updatePolicy, {
        roles: [{ role, mode } as any],
      });
      toast.show(t("access-policy-updated"), t("access-policy-updated"), {
        variant: "success",
      });
      await fetchAccessStatus();
    } catch (e: any) {
      console.error("failed to update policy", e);
      toast.show(
        t("access-policy-update-failed"),
        e?.message ?? t("access-policy-update-failed"),
        { variant: "error" },
      );
    } finally {
      setBusy(false);
    }
  };

  const addGrant = async () => {
    const trimmed = subject.trim().replace(/^@/, "");
    if (!agent || !trimmed) return;
    setBusy(true);
    try {
      await agent.client.call(place.stream.access.createGrant, {
        subject: trimmed,
        role: newRole as any,
        note: note.trim() || undefined,
      });
      toast.show(
        t("access-grant-created", { subject: trimmed }),
        t("access-grant-created", { subject: trimmed }),
        { variant: "success" },
      );
      setSubject("");
      setNote("");
      await Promise.all([loadGrants(), fetchAccessStatus()]);
    } catch (e: any) {
      console.error("failed to create grant", e);
      toast.show(
        t("access-grant-create-failed"),
        e?.message ?? t("access-grant-create-failed"),
        { variant: "error" },
      );
    } finally {
      setBusy(false);
    }
  };

  const deleteGrant = async (uri: string) => {
    if (!agent) return;
    setBusy(true);
    try {
      await agent.client.call(place.stream.access.deleteGrant, {
        uri: uri as any,
      });
      toast.show(t("access-grant-deleted"), t("access-grant-deleted"), {
        variant: "success",
      });
      setPendingDelete(null);
      await Promise.all([loadGrants(), fetchAccessStatus()]);
    } catch (e: any) {
      console.error("failed to delete grant", e);
      toast.show(
        t("access-grant-delete-failed"),
        e?.message ?? t("access-grant-delete-failed"),
        { variant: "error" },
      );
    } finally {
      setBusy(false);
    }
  };

  if (!canManage) {
    return (
      <View style={[zero.layout.flex.align.center, zero.px[16], zero.py[24]]}>
        <Text>{t("access-admin-required")}</Text>
      </View>
    );
  }

  const visibleGrants = (grants ?? []).filter(
    (g) => roleFilter === "all" || g.role === roleFilter,
  );
  const viewerMode = accessStatus?.policy.viewer;

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <MenuContainer>
            <View style={[zero.gap.all[2]]}>
              <Text size="2xl" weight="bold">
                {t("access-admin")}
              </Text>
              <Text color="muted">{t("access-admin-description")}</Text>
            </View>

            {busy && (
              <View style={[zero.layout.flex.align.center, zero.py[4]]}>
                <ActivityIndicator />
              </View>
            )}

            <MenuLabel>{t("access-policy")}</MenuLabel>
            <MenuGroup>
              {POLICY_ROLES.map((role, i) => (
                <View key={role}>
                  {i > 0 && <MenuSeparator />}
                  <MenuItem>
                    <SettingsRowItem>
                      <View style={[zero.gap.all[2], { flex: 1 }]}>
                        <Text size="sm" weight="semibold">
                          {t(`access-role-${role}`)}
                        </Text>
                        <Text size="xs" color="muted">
                          {t(`access-role-${role}-description`)}
                        </Text>
                        <SegmentedTabs
                          size="sm"
                          options={modeOptions}
                          value={accessStatus?.policy[role] ?? "open"}
                          onChange={(mode) => setMode(role, mode)}
                        />
                        {role === "viewer" &&
                          viewerMode !== undefined &&
                          viewerMode !== "open" && (
                            <Text
                              size="xs"
                              style={{ color: theme.colors.destructive }}
                            >
                              {t("access-viewer-warning")}
                            </Text>
                          )}
                      </View>
                    </SettingsRowItem>
                  </MenuItem>
                </View>
              ))}
            </MenuGroup>

            <MenuLabel>{t("access-grants")}</MenuLabel>
            <MenuGroup>
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("access-add-grant")}
                    </Text>
                    <Input
                      placeholder={t("access-subject-placeholder")}
                      value={subject}
                      onChangeText={setSubject}
                      autoCapitalize="none"
                      autoCorrect={false}
                    />
                    <Input
                      placeholder={t("access-note-placeholder")}
                      value={note}
                      onChangeText={setNote}
                    />
                    <SegmentedTabs
                      size="sm"
                      options={grantRoleOptions}
                      value={newRole}
                      onChange={setNewRole}
                    />
                    <Button
                      onPress={addGrant}
                      disabled={busy || !subject.trim()}
                      width="min"
                      style={{ height: 42 }}
                    >
                      {t("add")}
                    </Button>
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              <MenuItem>
                <SettingsRowItem>
                  <View style={[zero.gap.all[2], { flex: 1 }]}>
                    <Text size="sm" weight="semibold">
                      {t("access-filter")}
                    </Text>
                    <SegmentedTabs
                      size="sm"
                      options={filterOptions}
                      value={roleFilter}
                      onChange={setRoleFilter}
                    />
                  </View>
                </SettingsRowItem>
              </MenuItem>
              <MenuSeparator />
              {grantsLoading && grants === null ? (
                <View style={[zero.layout.flex.align.center, zero.py[4]]}>
                  <ActivityIndicator />
                </View>
              ) : visibleGrants.length === 0 ? (
                <MenuItem>
                  <SettingsRowItem>
                    <Text size="sm" color="muted">
                      {t("access-no-grants")}
                    </Text>
                  </SettingsRowItem>
                </MenuItem>
              ) : (
                visibleGrants.map((grant, i) => {
                  const key = grant.uri ?? `env:${grant.subject}:${grant.role}`;
                  const handle = profiles[grant.subject]?.handle;
                  const fromEnv = grant.source === "environment";
                  const confirming = !!grant.uri && pendingDelete === grant.uri;
                  return (
                    <View key={key}>
                      {i > 0 && <MenuSeparator />}
                      <MenuItem>
                        <SettingsRowItem>
                          <View style={[zero.gap.all[2], { flex: 1 }]}>
                            <View
                              style={[
                                zero.layout.flex.direction.row,
                                zero.layout.flex.align.center,
                                zero.gap.all[2],
                                { flexWrap: "wrap" },
                              ]}
                            >
                              <Text size="sm" weight="semibold">
                                {handle ? `@${handle}` : grant.subject}
                              </Text>
                              <Badge
                                label={t(`access-role-${grant.role}`)}
                                color={theme.colors.primary}
                              />
                              {fromEnv && (
                                <Badge
                                  label={t("access-source-environment")}
                                  color={theme.colors.text3}
                                  icon
                                />
                              )}
                            </View>
                            {handle && (
                              <Text size="xs" color="muted">
                                {grant.subject}
                              </Text>
                            )}
                            {grant.note && (
                              <Text size="xs" color="muted">
                                {grant.note}
                              </Text>
                            )}
                            {!fromEnv && grant.uri && (
                              <View
                                style={[
                                  zero.layout.flex.direction.row,
                                  zero.gap.all[2],
                                ]}
                              >
                                {confirming ? (
                                  <>
                                    <Button
                                      variant="danger"
                                      onPress={() => deleteGrant(grant.uri!)}
                                      disabled={busy}
                                      width="min"
                                      size="sm"
                                    >
                                      {t("access-confirm-remove")}
                                    </Button>
                                    <Button
                                      variant="secondary"
                                      onPress={() => setPendingDelete(null)}
                                      disabled={busy}
                                      width="min"
                                      size="sm"
                                    >
                                      {t("cancel")}
                                    </Button>
                                  </>
                                ) : (
                                  <Button
                                    variant="danger"
                                    onPress={() => setPendingDelete(grant.uri!)}
                                    disabled={busy}
                                    width="min"
                                    size="sm"
                                  >
                                    {t("access-remove")}
                                  </Button>
                                )}
                              </View>
                            )}
                          </View>
                        </SettingsRowItem>
                      </MenuItem>
                    </View>
                  );
                })
              )}
            </MenuGroup>
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}

function Badge({
  label,
  color,
  icon,
}: {
  label: string;
  color: string;
  icon?: boolean;
}) {
  return (
    <View
      style={{
        flexDirection: "row",
        alignItems: "center",
        gap: 4,
        paddingHorizontal: 8,
        paddingVertical: 2,
        borderRadius: 999,
        borderWidth: 1,
        borderColor: color,
      }}
    >
      {icon && <Lock size={11} color={color} />}
      <Text size="xs" style={{ color }}>
        {label}
      </Text>
    </View>
  );
}
