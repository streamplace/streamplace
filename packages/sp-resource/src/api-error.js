
const DEFAULT_CODES = {
  403: "FORBIDDEN"
};


/**
 * Can be called as
 * new APIError({
 *   message: "Bad thing happened",
 *   status: 500,
 *   code: "ERR_BAD_THING"
 * })
 *
 * or, shorthand:
 *
 * new APIError(403, "ur not allowed loser")
 */
export default class APIError extends Error {
  constructor(...args) {
    let {message, code, status} = args[0];
    if (args.length === 1 && typeof args[0] === "object") {
      if (!message || !code || !status) {
        throw new Error("Missing required parameters");
      }
    }
    else if (args.length > 1 && typeof args[0] === "number" && typeof args[1] === "string") {
      [status, message, code] = args;
    }
    else {
      throw new Error("Invalid invocation of APIError", args);
    }
    if (typeof message !== "string") {
      throw new Error("Invalid format for message");
    }
    if (typeof status !== "number" || status < 400 || status > 599) {
      throw new Error("Invalid HTTP status");
    }
    const shortMessage = message;
    const descriptiveMessage = `[${status}] ${code} - ${message}`;
    super(`[${status}] ${code} - ${message}`);
    this._shortMessage = shortMessage;
    this._descriptiveMessage = descriptiveMessage;
    this.code = code;
    this.status = status;
  }
}
