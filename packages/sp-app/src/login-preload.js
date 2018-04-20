// In renderer process (web page).
import { ipcRenderer } from "electron";
let params = new URL(document.location).searchParams;
let logout = params.get("electronLogout");

if (logout === "true") {
  Object.keys(window.localStorage).forEach(key => {
    window.localStorage.removeItem(key);
  });
  // remove the query params so it doesn't happen again when we log in
  window.history.replaceState({}, "", window.location.pathname);
}
window.streamplaceElectronCallback = details => {
  if (details) {
    ipcRenderer.send("logged-in", details);
  } else {
    ipcRenderer.send("not-logged-in");
  }
};
