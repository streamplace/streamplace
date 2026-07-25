import { Platform } from "react-native";

export default function clearQueryParams(par = ["iss", "state", "code"]) {
  if (Platform.OS !== "web") {
    return;
  }
  const u = new URL(document.location.href);
  const params = new URLSearchParams(u.search);
  if (u.search === "") {
    return;
  }
  par.forEach((p) => params.delete(p));
  u.search = params.toString();
  window.history.replaceState(null, "", u.toString());
}
