import { Keyboard, Pressable } from "react-native";
import { useKeyboardSlide } from "../../../hooks";
import * as atoms from "../../../lib/theme/atoms";
import { Input, Text, View } from "../../ui";
const { gap, h, layout, mt, p, position, px, py, sizes, w } = atoms;

type InputPanelProps = {
  title: string | undefined;
  setTitle: (title: string) => void;
  toggleGoLive: () => void;
  isLive: boolean;
  toggleStopStream?: () => void;
};

export function InputPanel({
  title,
  setTitle,
  toggleGoLive,
  isLive,
  toggleStopStream,
}: InputPanelProps) {
  const { slideKeyboard } = useKeyboardSlide();
  return (
    <View
      style={[
        layout.position.absolute,
        h.percent[30],
        position.bottom[0],
        w.percent[100],
        layout.flex.center,
        { transform: [{ translateY: slideKeyboard }] },
      ]}
    >
      <View
        style={[
          layout.flex.column,
          gap.all[2],
          sizes.maxWidth[80],
          { padding: 10 },
        ]}
      >
        {!isLive && (
          <View backgroundColor="rgba(64,64,64,0.8)" borderRadius={12}>
            <Input
              value={title}
              onChange={setTitle}
              placeholder="Enter stream title"
              onEndEditing={Keyboard.dismiss}
            />
          </View>
        )}
        {isLive ? (
          <View style={[layout.flex.center]}>
            <Pressable
              onPress={toggleStopStream}
              style={[
                px[4],
                py[2],
                layout.flex.row,
                layout.flex.center,
                gap.all[1],
                {
                  backgroundColor: "rgba(64,64,64, 0.8)",
                  borderRadius: 12,
                },
              ]}
            >
              <View
                style={[
                  p[2],
                  {
                    backgroundColor: "rgba(256,0,0, 0.8)",
                    borderRadius: 12,
                  },
                ]}
              />
              <Text center>Stop Stream</Text>
            </Pressable>
          </View>
        ) : (
          <View style={[layout.flex.center]}>
            <Pressable
              onPress={toggleGoLive}
              style={[
                px[4],
                py[2],
                layout.flex.row,
                layout.flex.center,
                gap.all[1],
                {
                  backgroundColor: "rgba(64,64,64, 0.8)",
                  borderRadius: 12,
                },
              ]}
            >
              <View
                style={[
                  p[2],
                  {
                    backgroundColor: "rgba(256,0,0, 0.8)",
                    borderRadius: 12,
                  },
                ]}
              />
              <Text center>Go Live</Text>
            </Pressable>
            <Text color="muted" size="xs" style={[mt[2]]}>
              We'll announce that you're live on Bluesky.
            </Text>
          </View>
        )}
      </View>
    </View>
  );
}
