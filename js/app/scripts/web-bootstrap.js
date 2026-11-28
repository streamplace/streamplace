// No React, Expo, or app imports: this runs before any application scripts.
const status = document.getElementById("bootstrap-status");
const spinner = document.getElementById("bootstrap-spinner");
const retry = document.getElementById("bootstrap-retry");
const scripts = JSON.parse(
  document.getElementById("bootstrap-scripts").textContent,
);
let loaded = 0;

if ("serviceWorker" in navigator) {
  const CONTROLLED_KEY = "streamplace-service-worker-controlled";
  let controlled = !!navigator.serviceWorker.controller;
  try {
    if (controlled) sessionStorage.setItem(CONTROLLED_KEY, "1");
    else controlled = sessionStorage.getItem(CONTROLLED_KEY) === "1";
  } catch (e) {
    // Storage is unavailable; trust this document's controller.
  }
  let refreshing = false;
  const early = () => performance.now() < 3000;
  navigator.serviceWorker.addEventListener("controllerchange", () => {
    try {
      sessionStorage.setItem(CONTROLLED_KEY, "1");
    } catch (e) {
      // Storage is unavailable; this document still has its controller.
    }
    if (controlled && early() && !refreshing) {
      refreshing = true;
      location.reload();
    }
  });
  navigator.serviceWorker
    .register("/sw.js", { updateViaCache: "none" })
    .then(() => navigator.serviceWorker.getRegistration())
    .then((registration) => {
      if (!registration) return;
      const scheduled = new WeakSet();
      const activate = (worker) => {
        if (!worker || !controlled || scheduled.has(worker)) return;
        scheduled.add(worker);
        const post = () => {
          if (early()) worker.postMessage({ type: "SKIP_WAITING" });
        };
        const requestIdle = () => {
          const active =
            navigator.serviceWorker.controller || registration.active;
          if (!active) {
            post();
            return;
          }
          const onIdle = (event) => {
            const type = event.data && event.data.type;
            if (type === "IDLE_ACK") {
              clearTimeout(timer);
            } else if (type === "IDLE") {
              clearTimeout(timer);
              navigator.serviceWorker.removeEventListener("message", onIdle);
              post();
            }
          };
          // A worker from a previous deploy never acknowledges; do not wait forever.
          const timer = setTimeout(() => {
            navigator.serviceWorker.removeEventListener("message", onIdle);
            post();
          }, 500);
          navigator.serviceWorker.addEventListener("message", onIdle);
          active.postMessage({ type: "WHEN_IDLE" });
        };
        // Ask after this document's loads reach the old worker; it answers
        // once its fetch and cache work has settled.
        if (document.readyState === "complete") {
          requestIdle();
        } else {
          window.addEventListener("load", requestIdle, { once: true });
        }
      };
      const watched = new WeakSet();
      const watch = (worker) => {
        if (!worker || watched.has(worker)) return;
        watched.add(worker);
        worker.addEventListener("statechange", () => {
          if (worker.state === "installed") activate(worker);
        });
      };
      // Re-read the registration: register() can resolve before an existing
      // waiting worker reaches this document's registration object.
      registration.addEventListener("updatefound", () =>
        watch(registration.installing),
      );
      activate(registration.waiting);
      watch(registration.installing);
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
