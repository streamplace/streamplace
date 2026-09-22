#!/usr/bin/env node
// Breaks reference cycles in the generated OpenAPI document before the docs
// build. A schema that refers to itself, directly or through other schemas
// (place.stream.chat.defs#messageView's replyTo is a messageView), sends
// starlight-openapi's schema renderer into unbounded recursion once an
// operation references it; the plugin version that handles recursive
// schemas needs a newer Astro and Starlight than this site runs. A
// depth-first walk over the schema graph replaces each $ref that points
// back at a schema still being expanded (a back edge) with a plain object
// naming that schema, so the page shows "object" with a description where
// it would otherwise overflow the stack. Deterministic: schemas and keys
// are walked in document order, so the same input gives the same output.
//
// Usage: node openapi-decycle.mjs <openapi.json>   (rewritten in place)
import { readFileSync, writeFileSync } from "node:fs";

const file = process.argv[2];
if (!file) {
  console.error("usage: openapi-decycle.mjs <openapi.json>");
  process.exit(2);
}
const doc = JSON.parse(readFileSync(file, "utf8"));
const schemas = doc.components?.schemas ?? {};
const PREFIX = "#/components/schemas/";

const onStack = new Set();
const done = new Set();
let broken = 0;

function stub(name) {
  return {
    type: "object",
    description: `A ${name} (recursive reference; see the schema of the same name).`,
  };
}

// walk mutates `node` in place: a child that is a $ref to a schema on the
// stack is replaced by a stub; any other $ref is followed (once).
function walk(node) {
  if (!node || typeof node !== "object") return;
  const entries = Array.isArray(node)
    ? node.map((v, i) => [i, v])
    : Object.entries(node);
  for (const [k, v] of entries) {
    if (v && typeof v === "object" && typeof v.$ref === "string" && v.$ref.startsWith(PREFIX)) {
      const target = v.$ref.slice(PREFIX.length);
      if (onStack.has(target)) {
        node[k] = stub(target);
        broken++;
      } else {
        visit(target);
      }
      continue;
    }
    walk(v);
  }
}

function visit(name) {
  if (done.has(name) || onStack.has(name) || !schemas[name]) return;
  onStack.add(name);
  walk(schemas[name]);
  onStack.delete(name);
  done.add(name);
}

for (const name of Object.keys(schemas)) visit(name);
// Operations reference schemas but are never referenced back, so a walk
// from each path only follows refs into schemas already made acyclic.
if (doc.paths) walk(doc.paths);

writeFileSync(file, JSON.stringify(doc, null, 2) + "\n");
console.log(`openapi-decycle: ${broken} recursive reference(s) broken in ${file}`);
