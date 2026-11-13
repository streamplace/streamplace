// No React, Expo, or app imports: this runs before any application scripts.
const status = document.getElementById("bootstrap-status");
const spinner = document.getElementById("bootstrap-spinner");
const retry = document.getElementById("bootstrap-retry");
const scripts = JSON.parse(
  document.getElementById("bootstrap-scripts").textContent,
);
let loaded = 0;

if ("serviceWorker" in navigator) {
  const controlled = !!navigator.serviceWorker.controller;
  let refreshing = false;
  const early = () => performance.now() < 3000;
  navigator.serviceWorker.addEventListener("controllerchange", () => {
    if (controlled && early() && !refreshing) {
      refreshing = true;
      location.reload();
    }
  });
  navigator.serviceWorker
    .register("/sw.js", { updateViaCache: "none" })
    .then((registration) => {
      const activate = (worker) => {
        if (worker && navigator.serviceWorker.controller && early()) {
          worker.postMessage({ type: "SKIP_WAITING" });
        }
      };
      const watch = () => {
        const worker = registration.installing;
        if (!worker) return;
        worker.addEventListener("statechange", () => {
          if (worker.state === "installed") activate(worker);
        });
      };
      activate(registration.waiting);
      watch();
      registration.addEventListener("updatefound", watch);
    })
    .catch((error) => console.warn("Streamplace service worker:", error));
}

async function boot() {
  retry.hidden = true;
  retry.disabled = true;
  spinner.removeAttribute("hidden");
  status.hidden = true;
  status.textContent = "";
  try {
    // Keep Expo's runtime/common/entry order. On retry, don't replay scripts
    // that already ran: in particular, reinitializing Metro loses its modules.
    for (; loaded < scripts.length; loaded++) {
      await new Promise((resolve, reject) => {
        const script = document.createElement("script");
        script.src = scripts[loaded];
        script.onload = () => {
          script.remove();
          resolve();
        };
        script.onerror = () => {
          script.remove();
          reject(new Error("Could not download " + script.src));
        };
        document.head.appendChild(script);
      });
    }
    // React replaces the bootstrap inside #root when its first render commits.
  } catch (error) {
    console.error("Streamplace bootstrap:", error);
    spinner.setAttribute("hidden", "");
    status.textContent = "Could not load Streamplace. Check your connection.";
    status.hidden = false;
    retry.disabled = false;
    retry.hidden = false;
  }
}

retry.addEventListener("click", boot);
// Let the lightweight HTML paint before downloading/evaluating the app.
requestAnimationFrame(() => requestAnimationFrame(boot));
