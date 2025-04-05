import {
  createChatProfileRecord,
  getChatProfileRecordFromPDS,
  selectChatProfile,
} from "features/bluesky/blueskySlice";
import { PlaceStreamChatProfile } from "lexicons";
import { useEffect, useState } from "react";
import ColorPicker, {
  HueSlider,
  Panel1,
  Preview,
  Swatches,
} from "reanimated-color-picker";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, H2, useTheme, View } from "tamagui";

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

  console.log(parts);
  return {
    red: parseInt(parts[0].trim(), 10),
    green: parseInt(parts[1].trim(), 10),
    blue: parseInt(parts[2].trim(), 10),
  };
}

export default function NameColorPicker() {
  const theme = useTheme();
  const dispatch = useAppDispatch();
  const chatProfile = useAppSelector(selectChatProfile);
  const [color, setColor] = useState(theme.accentColor.val);
  useEffect(() => {
    if (!chatProfile?.profile) {
      dispatch(getChatProfileRecordFromPDS());
    }
    if (chatProfile?.profile && chatProfile?.profile.color) {
      const { red, green, blue } = chatProfile.profile.color;
      setColor(`rgb(${red}, ${green}, ${blue})`);
    }
  }, [!chatProfile?.profile]);
  // onCompleteJS={(x) => setColor(x.rgb)}
  return (
    <View>
      <H2
        textAlign="center"
        color={color}
        // textShadowColor="white"
        // textShadowOffset={{ width: 0, height: 0 }}
        // textShadowRadius={3}
      >
        Change Name Color
      </H2>
      <ColorPicker value={color} onCompleteJS={(x) => setColor(x.rgb)}>
        <Preview />
        <Panel1 />
        <HueSlider />
        <Swatches />
      </ColorPicker>
      <Button
        backgroundColor="$accentColor"
        onPress={() => {
          dispatch(createChatProfileRecord(parseRgbString(color)));
        }}
      >
        Save
      </Button>
    </View>
  );
}
