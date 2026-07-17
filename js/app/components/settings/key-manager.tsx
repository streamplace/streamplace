import { useNavigation } from "@react-navigation/native";
import { EmptyState, EmptyStateTile } from "components/empty-state";
import Loading from "components/loading/loading";
import { useEffect, useState } from "react";
import { ActivityIndicator, ScrollView, View } from "react-native";
import { useStore } from "store";
import { useKeyRecords } from "store/hooks";
import { PlaceStreamKey } from "streamplace";
import { timeAgo } from "utils/timeAgo";

import { Button, IconButton, Text, useTheme } from "@streamplace/components";
import { fontFamilies } from "@streamplace/components/src/lib/theme/tokens";
import { KeyRound, RefreshCw, Trash2 } from "lucide-react-native";
import { useTranslation } from "react-i18next";

import { SettingsViewHeader } from "./components/settings-view-header";

function KeyRow({
  keyRecord,
  rkey,
  deleteKeyRecord,
  isDeleting,
  isLast,
}: {
  keyRecord: PlaceStreamKey.Record;
  rkey: string;
  deleteKeyRecord: (rkey: string) => void;
  isDeleting: boolean;
  isLast: boolean;
}) {
  const { theme } = useTheme();
  return (
    <View
      style={{
        flexDirection: "row",
        justifyContent: "space-between",
        alignItems: "center",
        gap: 12,
        paddingVertical: 14,
        paddingHorizontal: 16,
        borderBottomWidth: isLast ? 0 : 1,
        borderBottomColor: theme.colors.borderSubtle,
        opacity: isDeleting ? 0.5 : 1,
        pointerEvents: isDeleting ? "none" : "auto",
      }}
    >
      <View style={{ flex: 1, gap: 4 }}>
        {keyRecord?.signingKey && (
          <Text
            style={{
              fontFamily: fontFamilies.monoRegular,
              fontSize: 12,
              color: theme.colors.text1,
            }}
            numberOfLines={1}
            ellipsizeMode="middle"
          >
            {keyRecord?.signingKey}
          </Text>
        )}
        {keyRecord?.createdAt && (
          <Text style={{ fontSize: 12, color: theme.colors.text3 }}>
            made {timeAgo(new Date(keyRecord.createdAt))}{" "}
            {keyRecord.createdBy && "by " + keyRecord.createdBy}
          </Text>
        )}
      </View>
      <IconButton
        variant="ghost"
        size="sm"
        accessibilityLabel="Delete key"
        onPress={() => deleteKeyRecord(rkey)}
        disabled={isDeleting}
      >
        {isDeleting ? (
          <ActivityIndicator size="small" color={theme.colors.danger} />
        ) : (
          <Trash2 size={16} color={theme.colors.danger} />
        )}
      </IconButton>
    </View>
  );
}

export default function KeyManager() {
  const { theme } = useTheme();
  const deleteStreamKeyRecord = useStore(
    (state) => state.deleteStreamKeyRecord,
  );
  const getStreamKeyRecords = useStore((state) => state.getStreamKeyRecords);
  const keyObj = useKeyRecords();
  const keyRecords = keyObj?.records || null;
  const navigation = useNavigation();
  const { t } = useTranslation("settings");

  const [deletingKeys, setDeletingKeys] = useState<Set<string>>(new Set());
  const deleteKeyRecord = (rkey: string) => {
    if (deletingKeys.has(rkey)) return; // Prevent double deletes
    setDeletingKeys((prev) => new Set(prev).add(rkey));
    deleteStreamKeyRecord(rkey).finally(() => {
      setDeletingKeys((prev) => {
        const newSet = new Set(prev);
        newSet.delete(rkey);
        return newSet;
      });
    });
  };

  useEffect(() => {
    // delay 500ms to allow the screen to render
    setTimeout(() => {
      getStreamKeyRecords();
    }, 500);
  }, []);

  navigation.setOptions({ title: t("key-manager") });

  const isLoading = keyRecords === null || keyObj === null;
  const isEmpty = !isLoading && keyRecords.records.length === 0;

  return (
    <ScrollView contentContainerStyle={{ flexGrow: 1, alignItems: "center" }}>
      <View
        style={{
          paddingHorizontal: 32,
          paddingVertical: 24,
          maxWidth: 500,
          width: "100%",
          flex: 1,
        }}
      >
        <SettingsViewHeader
            title={t("your-stream-pubkeys")}
            description={t("pubkey-description")}
            action={
              <Button
                size="sm"
                width="min"
                variant="secondary"
                leftIcon={<RefreshCw size={16} />}
                onPress={() => getStreamKeyRecords()}
              >
                {t("refresh")}
              </Button>
            }
          />

          {isLoading ? (
            <View
              style={{
                flex: 1,
                minHeight: 320,
                justifyContent: "center",
                alignItems: "center",
              }}
            >
              <Loading />
            </View>
          ) : isEmpty ? (
            <EmptyState
              illustration={<EmptyStateTile icon={KeyRound} />}
              title={t("no-keys")}
              subtitle="Stream signing keys are generated when you go live. Head to the Live Dashboard to start streaming and create your first key."
              action={
                <Button
                  variant="secondary"
                  size="sm"
                  width="min"
                  onPress={() => (navigation as any).navigate("LiveDashboard")}
                >
                  {t("go-to-dashboard")}
                </Button>
              }
            />
          ) : (
            <>
              <View
                style={{
                  backgroundColor: theme.colors.surface1,
                  borderWidth: 1,
                  borderColor: theme.colors.borderSubtle,
                  borderRadius: theme.borderRadius.lg,
                  overflow: "hidden",
                }}
              >
                {keyRecords.records.map((keyRecord, index) => {
                  const rkey = keyRecord.uri.split("/").pop() as string;
                  return (
                    <KeyRow
                      key={rkey}
                      rkey={rkey}
                      keyRecord={keyRecord.value as any}
                      deleteKeyRecord={deleteKeyRecord}
                      isDeleting={deletingKeys.has(rkey)}
                      isLast={index === keyRecords.records.length - 1}
                    />
                  );
                })}
              </View>
              <Text size="sm" color="muted" style={{ marginTop: 12 }}>
                {t("keys-count", { count: keyRecords.records.length })}
              </Text>
            </>
          )}
        </View>
    </ScrollView>
  );
}
