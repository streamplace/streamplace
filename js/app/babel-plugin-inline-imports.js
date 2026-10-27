// import("x") -> Promise.resolve().then(() => require("x")), for the libraries
// behind Sign in with Ethereum (babel.config.js scopes it).
//
// Expo's web serializer moves every module that two async chunks share into
// __common.js, which the entry loads eagerly. viem lazy-loads a few modules
// of its own (CCIP lookups, secp256k1, WebSocket) that share much of its code
// with the rest of it, so left alone a few hundred KB of viem land in
// __common and every page load pays for them. Inlined, all of it stays in the
// one async chunk the login form import()s (components/login/
// ethereum-sign-in.web.ts).
function inlineImports({ types: t }) {
  return {
    visitor: {
      CallExpression(path) {
        if (!t.isImport(path.node.callee)) return;
        path.replaceWith(
          t.callExpression(
            t.memberExpression(
              t.callExpression(
                t.memberExpression(
                  t.identifier("Promise"),
                  t.identifier("resolve"),
                ),
                [],
              ),
              t.identifier("then"),
            ),
            [
              t.arrowFunctionExpression(
                [],
                t.callExpression(t.identifier("require"), path.node.arguments),
              ),
            ],
          ),
        );
      },
    },
  };
}

module.exports = inlineImports;
