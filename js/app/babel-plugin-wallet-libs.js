const path = require("path");

// Makes the WalletConnect libraries behind Sign in with Ethereum fit in the
// one lazy chunk the login form import()s (components/login/
// ethereum-sign-in.web.ts). babel.config.js applies it to those packages only.
//
// import("x") -> Promise.resolve().then(() => require("x")): Expo's web
// serializer moves every module two async chunks share into __common.js,
// which the entry loads eagerly. These libraries lazy-load a dozen views and
// optional connectors that share most of their code, so left alone all of it
// lands in __common and every page load pays for it. An import() that doesn't
// resolve as a require stays as it is: an optional dependency that isn't
// installed, which Metro tolerates only as an async import in a try, or an
// "import"-condition-only export (phosphor's icons).
//
// import.meta: Metro chunks are classic scripts, where import.meta is a
// syntax error that stops the whole chunk. Their ESM builds of valtio and
// zustand only read import.meta.env.MODE.
function walletLibs({ types: t }) {
  return {
    visitor: {
      CallExpression(p, state) {
        const [source] = p.node.arguments;
        if (!t.isImport(p.node.callee) || !t.isStringLiteral(source)) return;
        try {
          require.resolve(source.value, {
            paths: [path.dirname(state.filename)],
          });
        } catch {
          return;
        }
        p.replaceWith(
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
                t.callExpression(t.identifier("require"), [source]),
              ),
            ],
          ),
        );
      },
      MetaProperty(p) {
        if (p.node.meta.name !== "import") return;
        const parent = p.parentPath;
        if (
          parent.isMemberExpression({ object: p.node }) &&
          t.isIdentifier(parent.node.property, { name: "env" })
        ) {
          // { MODE: process.env.NODE_ENV }, which Metro inlines
          parent.replaceWith(
            t.objectExpression([
              t.objectProperty(
                t.identifier("MODE"),
                t.memberExpression(
                  t.memberExpression(
                    t.identifier("process"),
                    t.identifier("env"),
                  ),
                  t.identifier("NODE_ENV"),
                ),
              ),
            ]),
          );
          return;
        }
        // anything else gets Expo's own import.meta stand-in
        p.replaceWith(
          t.memberExpression(
            t.identifier("globalThis"),
            t.identifier("__ExpoImportMetaRegistry"),
          ),
        );
      },
    },
  };
}

module.exports = walletLibs;
