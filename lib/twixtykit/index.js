
// Eventually this will contain a lot of rad shared UI logic. For now it just contains the error handler.

export default {
  info: function(...args) {
    /*eslint-disable no-console */
    console.info(args);
  },
  error: function(...args) {
    /*eslint-disable no-console */
    console.error(args);
  },
};
