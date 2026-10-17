// RFC 3339 with the device's offset at this instant, including DST.
// Unlike toISOString(), this preserves the sender's local wall-clock time.
export function formatLocalTimestamp(date: Date = new Date()): string {
  const offset = date.getTimezoneOffset();
  const local = new Date(date.getTime() - offset * 60_000)
    .toISOString()
    .slice(0, -1);
  const magnitude = Math.abs(offset);
  const hours = String(Math.floor(magnitude / 60)).padStart(2, "0");
  const minutes = String(magnitude % 60).padStart(2, "0");
  return `${local}${offset > 0 ? "-" : "+"}${hours}:${minutes}`;
}
