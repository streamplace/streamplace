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

/** Return a stable accent color for an AT Protocol identity. */
export function getDidAccentColor(did: string): string {
  let hash = 0;
  for (let i = 0; i < did.length; i++) {
    hash = (hash * 31 + did.charCodeAt(i)) | 0;
  }
  const hue = ((hash % 360) + 360) % 360;
  return `hsl(${hue} 40% 50%)`;
}
