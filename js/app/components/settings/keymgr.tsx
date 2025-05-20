import { YStack, XStack, Text, Separator, Button, ScrollView } from "tamagui";
import { useEffect } from "react";
import { Dices, X } from "@tamagui/lucide-icons";
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
  deleteKeyRecord,
}: {
  keyRecord: PlaceStreamKey.Record;
  rkey: string;
  deleteKeyRecord: (rkey: string) => void;
}) {
  return (
    <XStack
      style={{
        justifyContent: "space-between",
        alignItems: "center",
      }}
      gap="$4"
    >
      <XStack gap="$4">
        {keyRecord?.signingKey && (
          <Text
            fontFamily="$mono"
            fontSize="$2"
            $sm={{ width: "$14" }}
            ellipse
            numberOfLines={1}
          >
            {keyRecord?.signingKey}
          </Text>
        )}
        {keyRecord?.createdAt && (
          <Text fontSize="$2" f={1}>
            made {timeAgo(new Date(keyRecord.createdAt))}
          </Text>
        )}
      </XStack>
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
  const keyRecords = useAppSelector(selectKeyRecords);

  const deleteKeyRecord = (rkey: string) => {
    dispatch(deleteStreamKeyRecord({ rkey }));
    dispatch(getStreamKeyRecords());
  };

  useEffect(() => {
    dispatch(getStreamKeyRecords());
  }, []);

  return (
    <ScrollView justifyContent="flex-start" alignItems="center">
      <YStack f={1} p="$4" gap="$4" maxWidth={750}>
        {keyRecords === null ? (
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
            <YStack gap="$2">
              <Text fontSize="$8">Existing Pubkeys</Text>
              <Text fontSize="$2" color="$color.gray11Dark">
                Your private stream key is the secret credential you use to
                stream. Listed are the associated public keys.
              </Text>
              {keyRecords.records.map((keyRecord) => (
                <KeyRow
                  rkey={keyRecord.uri.split("/").pop() as string}
                  keyRecord={keyRecord.value as any}
                  deleteKeyRecord={deleteKeyRecord}
                />
              ))}
              <Text fontSize="$2" color="$color.gray11Dark">
                {keyRecords.records.length} key
                {keyRecords.records.length > 1 && "s"}
              </Text>
            </YStack>
            <Separator />

            <Text fontSize="$2" color="$color.gray11Dark">
              Go to the live dashboard to create a key.
            </Text>
          </>
        )}
      </YStack>
    </ScrollView>
  );
}
