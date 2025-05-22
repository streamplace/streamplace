import {
  YStack,
  XStack,
  Text,
  Separator,
  Button,
  ScrollView,
  View,
} from "tamagui";
import { useEffect } from "react";
import { RefreshCcw, X } from "@tamagui/lucide-icons";
import {
  deleteStreamKeyRecord,
  getStreamKeyRecords,
  selectKeyRecords,
} from "features/bluesky/blueskySlice";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { PlaceStreamKey } from "lexicons";
import Loading from "components/loading/loading";
import { timeAgo } from "utils/timeAgo";
import AQLink from "components/aqlink";

function KeyRow({
  keyRecord,
  rkey,
  by,
  deleteKeyRecord,
}: {
  keyRecord: PlaceStreamKey.Record;
  rkey: string;
  by: string;
  deleteKeyRecord: (rkey: string) => void;
}) {
  return (
    <XStack justifyContent="space-between" alignItems="stretch" gap="$4">
      <View
        flexDirection="row"
        $xs={{ flexDirection: "column", marginBottom: "$4" }}
        gap="$2"
      >
        {keyRecord?.signingKey && (
          <Text
            fontFamily="$mono"
            fontSize="$2"
            $xs={{ width: "$20" }}
            ellipse
            numberOfLines={1}
          >
            {keyRecord?.signingKey}
          </Text>
        )}
        {keyRecord?.createdAt && (
          <Text fontSize="$2" color="$color.gray11Dark" f={1}>
            made
            {by ? (
              <Text color="$color.gray11Dark">
                {" "}
                by <Text color="$color.gray12Dark">{by}</Text>
              </Text>
            ) : (
              ""
            )}{" "}
            <Text color="$color.gray12Dark">
              {timeAgo(new Date(keyRecord.createdAt))}
            </Text>
          </Text>
        )}
      </View>
      <Button
        aria-label="Delete"
        size="$3"
        aspectRatio={1 / 1}
        padding="$2"
        hoverStyle={{ backgroundColor: "#f46" }}
        onPress={() => deleteKeyRecord(rkey)}
      >
        <X />
      </Button>
    </XStack>
  );
}

export default function KeyManager() {
  const dispatch = useAppDispatch();
  const keyObj = useAppSelector(selectKeyRecords);
  const keyRecords = keyObj.records;

  const deleteKeyRecord = (rkey: string) => {
    dispatch(deleteStreamKeyRecord({ rkey }));
    dispatch(getStreamKeyRecords());
  };

  useEffect(() => {
    dispatch(getStreamKeyRecords());
  }, []);

  return (
    <ScrollView>
      <View justifyContent="center" alignItems="center">
        <YStack f={1} p="$4" gap="$6" maxWidth={700}>
          {keyObj.error ? (
            <>
              <Text fontSize="$6" color="$color.red">
                Encountered an error while getting stream keys: {keyObj.error}
              </Text>
              <Button
                aria-label="Refresh"
                size="$3"
                padding="$2"
                onPress={() => dispatch(getStreamKeyRecords())}
              >
                <RefreshCcw />
              </Button>
            </>
          ) : keyObj.loading == true || keyRecords === null ? (
            <Loading />
          ) : keyRecords.records.length === 0 ? (
            <>
              <Text mt="$8">No keys here!</Text>
              <AQLink to={{ screen: "LiveDashboard" }}>
                <Text fontSize="$2" color="$color.blue7Light">
                  Go to the live dashboard to create a key.
                </Text>
              </AQLink>
            </>
          ) : (
            <>
              <YStack
                gap="$2"
                borderBottomWidth={1}
                borderBottomColor="$color.gray3Dark"
                pb="$2"
                mb="$2"
              >
                <YStack
                  gap="$2"
                  borderBottomWidth={1}
                  borderBottomColor="$color.gray3Dark"
                  pb="$2"
                  mb="$2"
                >
                  <Text fontSize="$8">Your Stream Pubkeys</Text>
                  <Text fontSize="$2" color="$color.gray11Dark">
                    A pubkey is a pair to one of your stream keys. You can
                    revoke access for a specific stream key by revoking its
                    associated pubkey below.
                  </Text>
                </YStack>
                <YStack gap="$2">
                  {keyRecords.records.map((keyRecord) => (
                    <KeyRow
                      rkey={keyRecord.uri.split("/").pop() as string}
                      keyRecord={keyRecord.value as any}
                      by={keyRecord.value.createdBy as any}
                      deleteKeyRecord={deleteKeyRecord}
                    />
                  ))}
                </YStack>
                <Text fontSize="$2" color="$color.gray11Dark">
                  {keyRecords.records.length} key
                  {keyRecords.records.length > 1 && "s"}
                </Text>
              </YStack>

              <AQLink to={{ screen: "LiveDashboard" }}>
                <Text fontSize="$2" color="$color.blue7Light">
                  Go to the live dashboard to create a key.
                </Text>
              </AQLink>
            </>
          )}
        </YStack>
      </View>
    </ScrollView>
  );
}
