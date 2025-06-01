import { X } from "@tamagui/lucide-icons";
import {
  createChatProfileRecord,
  getChatProfileRecordFromPDS,
  selectChatProfile,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import { useEffect, useState } from "react";
import { Keyboard } from "react-native";
import ColorPicker, {
  HueSlider,
  Panel1,
  Preview,
  Swatches,
} from "reanimated-color-picker";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { PlaceStreamChatProfile } from "streamplace";
import { Button, H3, isWeb, Sheet, useTheme, View } from "tamagui";

/**
 * Parses an RGB color string and returns an object with red, green, and blue values
 * @param rgbString - RGB color string in the format "rgb(r,g,b)" or "rgba(r,g,b,a)"
 * @returns An object containing red, green, and blue values as numbers
 */
function parseRgbString(rgbString: string): PlaceStreamChatProfile.Color {
  // Check if the string is empty or not in the expected format
  if (
    !rgbString ||
    (!rgbString.startsWith("rgb(") && !rgbString.startsWith("rgba("))
  ) {
    throw new Error("Invalid color string (not rgb or rgba)");
  }
  // Extract the numbers from the string
  const numbersString = rgbString.replace(/^rgba?\(|\)$/g, "");
  const parts = numbersString.split(",");

  // Make sure we have at least 3 parts for r, g, b
  if (parts.length < 3) {
    throw new Error("Invalid color string (not enough parts)");
  }

  return {
    red: parseInt(parts[0].trim(), 10),
    green: parseInt(parts[1].trim(), 10),
    blue: parseInt(parts[2].trim(), 10),
  };
}

export default function NameColorPicker({
  children,
  text,
  buttonProps,
}: {
  children?: React.ReactNode;
  text?: (color: string) => React.ReactNode;
  buttonProps?: React.ComponentProps<typeof Button>;
}) {
  const theme = useTheme();
  const [open, setOpen] = useState(false);
  const dispatch = useAppDispatch();
  const chatProfile = useAppSelector(selectChatProfile);
  const [color, setColor] = useState(theme.accentColor?.val ?? "#bd6e86");
  const profile = useAppSelector(selectUserProfile);

  useEffect(() => {
    if (profile?.did && !chatProfile?.profile) {
      dispatch(getChatProfileRecordFromPDS());
    }
    if (chatProfile?.profile && chatProfile?.profile.color) {
      const { red, green, blue } = chatProfile.profile.color;
      setColor(`rgb(${red}, ${green}, ${blue})`);
    }
  }, [profile?.did, chatProfile?.profile?.color]);

  return (
    <View alignItems="center" flexDirection="row">
      <Button
        {...buttonProps}
        color={color}
        onPress={() => {
          if (!isWeb) {
            Keyboard.dismiss();
          }
          setOpen(true);
        }}
      >
        {text ? text(color) : "Change Name Color"}
      </Button>
      <Sheet
        // forceRemoveScrollEnabled={open}
        open={open}
        modal={true}
        onOpenChange={(open) => {
          setOpen(open);
          if (!open) {
            dispatch(getChatProfileRecordFromPDS());
          }
        }}
        dismissOnSnapToBottom
        disableDrag={true}
        zIndex={100_000}
        animation="medium"
      >
        <Sheet.Overlay
          animation="lazy"
          backgroundColor="$shadow6"
          enterStyle={{ opacity: 0 }}
          exitStyle={{ opacity: 0 }}
        />
        <Sheet.Frame>
          <View
            f={1}
            alignItems="stretch"
            justifyContent="center"
            padding="$4"
            paddingBottom="300"
            gap="$5"
            maxWidth={600}
            marginHorizontal="auto"
          >
            <Button
              position="absolute"
              top="$0"
              right="$0"
              onPress={(e) => {
                e.stopPropagation();
                setOpen(false);
              }}
              marginRight={-15}
              marginTop={5}
              backgroundColor="transparent"
            >
              <X />
            </Button>
            <H3 textAlign="center" color={color}>
              @{profile?.handle}
            </H3>
            <ColorPicker value={color} onCompleteJS={(x) => setColor(x.rgb)}>
              <Preview />
              <Panel1 />
              <HueSlider />
              <Swatches style={{ margin: 10 }} />
            </ColorPicker>
            <Button
              backgroundColor="$accentColor"
              onPress={() => {
                setOpen(false);
                dispatch(createChatProfileRecord(parseRgbString(color)));
              }}
            >
              Save
            </Button>
          </View>
        </Sheet.Frame>
      </Sheet>
    </View>
  );
}
