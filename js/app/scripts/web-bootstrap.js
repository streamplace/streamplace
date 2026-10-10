// No React, Expo, or app imports: this runs before any application scripts.
const status = document.getElementById("bootstrap-status");
const spinner = document.getElementById("bootstrap-spinner");
const retry = document.getElementById("bootstrap-retry");
const scripts = JSON.parse(
  document.getElementById("bootstrap-scripts").textContent,
);
let loaded = 0;

async function boot() {
  retry.hidden = true;
  retry.disabled = true;
  spinner.hidden = false;
  status.textContent = "Loading Streamplace";
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
    spinner.hidden = true;
    status.textContent = "Could not load Streamplace. Check your connection.";
    retry.disabled = false;
    retry.hidden = false;
  }
}

retry.addEventListener("click", boot);
// Let the lightweight HTML paint before downloading/evaluating the app.
requestAnimationFrame(() => requestAnimationFrame(boot));
