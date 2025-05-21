import { useEffect, useState } from "react";
import { Button, H3, Label, Paragraph, Text, TextArea, View } from "tamagui";
import { useToastController } from "@tamagui/toast";
import {
  selectNewLivestream,
  selectUserProfile,
  updateLivestreamRecord,
} from "features/bluesky/blueskySlice";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { useLiveUser } from "hooks/useLiveUser";
import { ScrollView } from "react-native";

export default function UpdateLivestream({
  playerId,
}: {
  playerId: string | null;
}) {
  const dispatch = useAppDispatch();
  const toast = useToastController();
  const userIsLive = useLiveUser();
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(false);
  const profile = useAppSelector(selectUserProfile);
  const newLivestream = useAppSelector(selectNewLivestream);

  useEffect(() => {
    if (newLivestream?.record) {
      toast.show("Livestream title updated", {
        message: newLivestream.record.title,
      });
      setTitle("");
    }
  }, [newLivestream?.record]);
  useEffect(() => {
    if (newLivestream?.error) {
      toast.show("Error updating livestream", {
        message: newLivestream.error,
      });
    }
  }, [newLivestream?.error]);
  const disabled = !userIsLive || loading || title === "";

  if (!playerId) {
    return (
      <View justifyContent="center" alignContent="center">
        <Text>
          Couldn't get the player ID. You may not have created an initial
          livestream record.
        </Text>
      </View>
    );
  }

  const handleSubmit = async () => {
    setLoading(true);
    try {
      await dispatch(
        updateLivestreamRecord({
          title,
          playerId,
        }),
      );
    } catch (error) {
      console.error("Error updating livestream:", error);
      toast.show("Error updating livestream", {
        message: String(error),
      });
    } finally {
      setLoading(false);
    }
  };

  const buttonText = loading
    ? "Loading..."
    : !userIsLive
      ? "Waiting for stream to start..."
      : "Update Livestream!";

  return (
    <ScrollView
      style={{ width: "60%" }}
      contentContainerStyle={{
        flexGrow: 1,
        justifyContent: "flex-start",
        paddingVertical: 40,
      }}
      showsVerticalScrollIndicator={false}
    >
      <H3 pl="$4">Change your Current Livestream Title</H3>
      <View w="100%" alignSelf="center" p="$4" justifyContent="center">
        <View f={2} minWidth={0} gap="$3">
          <Label asChild={true} display="flex">
            <View flexDirection="row" alignItems="center" w="100%">
              <Paragraph pb="$2" minWidth={100} textAlign="left">
                Streamer
              </Paragraph>
              <Paragraph pb="$2" fontWeight="bold">
                @{profile?.handle}
              </Paragraph>
            </View>
          </Label>
          <Label asChild={true}>
            <View flexDirection="row" alignItems="center" w="100%">
              <Paragraph pb="$2" minWidth={100} textAlign="left">
                Title
              </Paragraph>
              <View flex={1}>
                <TextArea
                  id="livestream-title"
                  value={title}
                  onChangeText={setTitle}
                  size="$4"
                  minHeight={100}
                  maxLength={140}
                  w="100%"
                />
              </View>
            </View>
          </Label>
          <Label asChild={true} mt="$-4">
            <View flexDirection="row" alignItems="center" w="100%">
              <Paragraph minWidth={100} textAlign="left"></Paragraph>
              <View flex={1}>
                <Text fontSize="$1" color="$gray11">
                  Updating will not send out notifications to viewers or create
                  a new social media post.
                </Text>
              </View>
            </View>
          </Label>
          <View w="100%" alignItems="center" mt="$-4">
            <Button
              disabled={disabled}
              opacity={disabled ? 0.5 : 1}
              size="$4"
              w="100%"
              onPress={handleSubmit}
            >
              {buttonText}
            </Button>
          </View>
        </View>
      </View>
    </ScrollView>
  );
}
