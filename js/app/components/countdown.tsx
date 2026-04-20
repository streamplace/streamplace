import { Text, zero } from "@streamplace/components";
import { fontFamilies } from "@streamplace/components/src/lib/theme/tokens";
import * as chrono from "chrono-node";
import { useEffect, useState } from "react";
import { View, useWindowDimensions } from "react-native";

interface CountdownProps {
  from?: string;
  to?: string;
  small?: boolean;
}

interface LabelBoxProps {
  children: React.ReactNode;
  small?: boolean;
}

const LabelBox = ({ children, small }: LabelBoxProps) => {
  return (
    <View
      style={[
        {
          borderColor: "white",
          borderWidth: 0,
          borderTopWidth: small ? 2 : 4,
          borderStyle: "solid",
        },
      ]}
    >
      <Text
        style={[
          small ? { fontSize: 18 } : { fontSize: 24 },
          small ? { lineHeight: 24 } : { lineHeight: 28 },
        ]}
      >
        {children}
      </Text>
    </View>
  );
};

export function Countdown({ from, to, small }: CountdownProps) {
  const [now, setNow] = useState(Date.now());
  const [dest, setDest] = useState<number | null>(null);
  const { width, height } = useWindowDimensions();

  useEffect(() => {
    if (from) {
      const fromDate = chrono.parseDate(from);
      if (fromDate === null) {
        throw new Error("could not parse from");
      }
      setDest(fromDate.getTime());
    } else if (to) {
      const toDate = chrono.parseDate(to);
      if (toDate === null) {
        throw new Error("could not parse to");
      }
      setDest(toDate.getTime());
    } else {
      throw new Error("must provide either from or to");
    }
  }, [from, to]);

  useEffect(() => {
    const tick = () => {
      if (!running) {
        return;
      }
      requestAnimationFrame(tick);
      setNow(Date.now());
    };
    let running = true;
    tick();
    return () => {
      running = false;
    };
  }, []);

  if (dest === null) {
    return <View />;
  }

  let diff = Math.abs(dest - now);
  if (to && now > dest) {
    diff = 0;
  } else if (from && now < dest) {
    diff = 0;
  }
  small = small ?? width <= 600;
  const [years, days, hrs, min, sec, ms] = toLabels(diff);

  const unitStyle = [
    zero.mx[small ? 2 : 4],
    zero.flex.values[0],
    { flexDirection: "column" },
  ];

  const timeTextStyle = [
    { fontFamily: fontFamilies.monoRegular },
    small ? { fontSize: 18 } : { fontSize: 128 },
    small ? { lineHeight: 24 } : { lineHeight: 40 },
  ];

  return (
    <View
      style={[
        { flexDirection: "row" },
        { alignSelf: small ? "auto" : "center" },
      ]}
    >
      <View style={[{ flexDirection: "row" }, { justifyContent: "flex-end" }]}>
        <View style={unitStyle}>
          <Text style={timeTextStyle}>{years}</Text>
          <LabelBox small={small}>YEARS</LabelBox>
        </View>
        <View style={unitStyle}>
          <Text style={timeTextStyle}>{days}</Text>
          <LabelBox small={small}>DAYS</LabelBox>
        </View>
        <View style={unitStyle}>
          <Text style={timeTextStyle}>{hrs}</Text>
          <LabelBox small={small}>HRS</LabelBox>
        </View>
      </View>
      <View style={[{ flexDirection: "row" }, { justifyContent: "flex-end" }]}>
        <View style={unitStyle}>
          <Text style={timeTextStyle}>{min}</Text>
          <LabelBox small={small}>MIN</LabelBox>
        </View>
        <View style={unitStyle}>
          <Text style={timeTextStyle}>{sec}</Text>
          <LabelBox small={small}>SEC</LabelBox>
        </View>
        <View style={unitStyle}>
          <Text style={timeTextStyle}>{ms}</Text>
          <LabelBox small={small}>MS</LabelBox>
        </View>
      </View>
    </View>
  );
}

const toLabels = (
  now: number,
): [string, string, string, string, string, string] => {
  const ms = now % 1000;
  now = Math.floor(now / 1000);

  const sec = now % 60;
  now = Math.floor(now / 60);

  const min = now % 60;
  now = Math.floor(now / 60);

  const hrs = now % 24;
  now = Math.floor(now / 24);

  const days = now % 365;
  now = Math.floor(now / 365);

  const years = now;

  return [
    pad(years, 4),
    pad(days, 3),
    pad(hrs, 2),
    pad(min, 2),
    pad(sec, 2),
    pad(ms, 3),
  ];
};

const pad = (num: number, n: number): string => {
  let str = `${num}`;
  while (str.length < n) {
    str = "0" + str;
  }
  return str;
};
