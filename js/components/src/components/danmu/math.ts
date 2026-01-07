export const mapRange = (
  num: number,
  inMin: number,
  inMax: number,
  outMin: number,
  outMax: number,
) => {
  return ((num - inMin) * (outMax - outMin)) / (inMax - inMin) + outMin;
};

export const between = (num: number, min: number, max: number) => {
  return Math.min(Math.max(num, min), max);
};

export const baseDuration = (
  message: { record: { text: string | any[] } },
  min: number,
  max: number,
) =>
  between(
    mapRange(Math.log(message.record.text.length) * 8, 1, 16, min, max),
    min,
    max,
  );

export const MIN_DURATION = 6000;
export const MAX_DURATION = 12000;
