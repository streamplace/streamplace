global.AbortSignal.timeout = function (ms) {
  const controller = new AbortController();

  setTimeout(() => {
    controller.abort(new Error(`signal timed out after ${ms} ms`));
  }, ms);

  return controller.signal;
};
global.AbortSignal.prototype.throwIfAborted = function () {
  const { aborted, reason = "Aborted" } = this;
  if (!aborted) return;
  throw reason;
};
