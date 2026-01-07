import { useNavigation } from "@react-navigation/native";
import AQLink from "components/aqlink";
import Loading from "components/loading/loading";
import { useEffect, useState } from "react";
import {
  ActivityIndicator,
  ScrollView,
  TouchableOpacity,
  View,
} from "react-native";
import { useStore } from "store";
import { useKeyRecords } from "store/hooks";
import { PlaceStreamKey } from "streamplace";
import { timeAgo } from "utils/timeAgo";

import { Text, zero } from "@streamplace/components";
import { X } from "lucide-react-native";
import { useTranslation } from "react-i18next";

function KeyRow({
  keyRecord,
  rkey,
  deleteKeyRecord,
  isDeleting,
}: {
  keyRecord: PlaceStreamKey.Record;
  rkey: string;
  deleteKeyRecord: (rkey: string) => void;
  isDeleting: boolean;
}) {
  return (
    <View
      style={[
        zero.layout.flex.row,
        zero.layout.flex.justify.between,
        zero.layout.flex.align.center,
        zero.gap.all[2],
        {
          opacity: isDeleting ? 0.5 : 1,
          pointerEvents: isDeleting ? "none" : "auto",
        },
      ]}
    >
      <View style={[zero.flex.values[1], zero.gap.all[1]]}>
        {keyRecord?.signingKey && (
          <Text
            style={[
              {
                fontFamily: "monospace",
                fontSize: 12,
                color: "#fff",
              },
            ]}
            numberOfLines={1}
            ellipsizeMode="middle"
          >
            {keyRecord?.signingKey}
          </Text>
        )}
        {keyRecord?.createdAt && (
          <Text style={[{ fontSize: 12, color: "#999" }]}>
            made {timeAgo(new Date(keyRecord.createdAt))}{" "}
            {keyRecord.createdBy && "by " + keyRecord.createdBy}
          </Text>
        )}
      </View>
      <TouchableOpacity
        style={[
          zero.h[6],
          zero.w[6],
          zero.r.md,
          zero.layout.flex.align.center,
          zero.layout.flex.justify.center,
          { backgroundColor: isDeleting ? "#666" : "#333" },
        ]}
        onPress={() => deleteKeyRecord(rkey)}
        disabled={isDeleting}
      >
        {isDeleting ? (
          <ActivityIndicator size="small" color="#fff" />
        ) : (
          <X size={16} color="#fff" />
        )}
      </TouchableOpacity>
    </View>
  );
}

export default function KeyManager() {
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

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[8]]}>
        <View style={[zero.py[2], { maxWidth: 500, width: "100%" }]}>
          {keyRecords === null || keyObj === null ? (
            <Loading />
          ) : keyRecords.records.length === 0 ? (
            <>
              <Text size="xl">{t("no-keys")}</Text>
              <AQLink to={{ screen: "LiveDashboard" }}>
                <Text size="lg" color="muted">
                  {t("go-to-dashboard")}
                </Text>
              </AQLink>
              <TouchableOpacity
                style={[
                  zero.px[12],
                  zero.py[12],
                  zero.r.md,
                  zero.layout.flex.align.center,
                  zero.layout.flex.justify.center,
                  { backgroundColor: "#333" },
                ]}
                onPress={() => getStreamKeyRecords()}
              >
                <Text size="lg">{t("refresh")}</Text>
              </TouchableOpacity>
            </>
          ) : keyObj.loading == true || keyRecords === null ? (
            <Loading />
          ) : keyRecords.records.length === 0 ? (
            <>
              <Text size="xl">{t("no-keys")}</Text>
              <AQLink to={{ screen: "LiveDashboard" }}>
                <Text size="lg" color="muted">
                  {t("go-to-dashboard")}
                </Text>
              </AQLink>
            </>
          ) : (
            <>
              <View style={[zero.mb[4]]}>
                <Text size="xl">{t("your-stream-pubkeys")}</Text>
                <Text size="lg" color="muted">
                  {t("pubkey-description")}
                </Text>
              </View>
              <View style={[zero.gap.all[4]]}>
                {keyRecords.records.map((keyRecord) => {
                  const rkey = keyRecord.uri.split("/").pop() as string;
                  return (
                    <KeyRow
                      key={rkey}
                      rkey={rkey}
                      keyRecord={keyRecord.value as any}
                      deleteKeyRecord={deleteKeyRecord}
                      isDeleting={deletingKeys.has(rkey)}
                    />
                  );
                })}
              </View>
              <Text size="lg" color="muted">
                {t("keys-count", { count: keyRecords.records.length })}
              </Text>
            </>
          )}
        </View>
      </View>
    </ScrollView>
  );
}
