/**
 * Convert a place.stream.chat.profile.Color to a CSS rgb() string.
 * Returns a default pink if no color is provided.
 */
export function getRgbColor(
  color?: { red: number; green: number; blue: number } | null,
): string {
  if (!color) return "#bd6e86";
  return `rgb(${color.red}, ${color.green}, ${color.blue})`;
}
