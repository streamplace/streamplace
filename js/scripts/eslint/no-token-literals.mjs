// Disallows hardcoded style literals in component code: raw hex colors,
// rgb()/rgba() strings, and raw palette-ramp indexing (colors.primary[500]).
//
// Every visual value should come from the theme (see the streamplace-design
// skill). A line carrying a `token-ok` comment is exempt, reserved for literals
// that must render when the theme is unavailable: crash screens, transparent
// overlay roots composited over OBS content, and brand-guideline swatches.

const RAMPS = new Set(
  "slate gray zinc neutral stone red orange amber yellow lime green emerald teal cyan sky blue indigo violet purple fuchsia pink rose primary destructive success warning".split(
    " ",
  ),
);

const HEX = /#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})\b/;
const RGB = /\brgba?\(/;
const ALLOW = "token-ok";

function literalKind(value) {
  if (typeof value !== "string") return null;
  if (HEX.test(value)) return "hex";
  if (RGB.test(value)) return "rgb";
  return null;
}

// Matches `colors.<ramp>[…]` and `theme.colors.<ramp>[…]`, i.e. the same
// computed-member shape however deeply `colors` is nested behind an object.
function isRampIndex(node) {
  const owner = node.object;
  if (!owner || owner.type !== "MemberExpression" || owner.computed)
    return false;
  if (
    owner.property?.type !== "Identifier" ||
    !RAMPS.has(owner.property.name)
  ) {
    return false;
  }
  const base = owner.object;
  if (base.type === "Identifier") return base.name === "colors";
  if (base.type === "MemberExpression" && !base.computed) {
    return (
      base.property?.type === "Identifier" && base.property.name === "colors"
    );
  }
  return false;
}

export default {
  meta: {
    type: "problem",
    docs: {
      description: "disallow hardcoded style literals in component code",
    },
    messages: {
      literal:
        'Hardcoded {{kind}} style literal "{{text}}". Read the value from the theme, or exempt the line with a // token-ok comment.',
    },
  },
  create(context) {
    const sourceCode = context.sourceCode ?? context.getSourceCode();

    // A token-ok comment exempts every line it spans, mirroring the previous
    // line-oriented check so trailing and inline comments both work.
    const exempt = new Set();
    for (const comment of sourceCode.getAllComments()) {
      if (!comment.value.includes(ALLOW)) continue;
      for (
        let line = comment.loc.start.line;
        line <= comment.loc.end.line;
        line++
      ) {
        exempt.add(line);
      }
    }

    const check = (node, kind, text) => {
      if (exempt.has(node.loc.start.line)) return;
      context.report({ node, messageId: "literal", data: { kind, text } });
    };

    return {
      Literal(node) {
        const kind = literalKind(node.value);
        if (kind) check(node, kind, String(node.value));
      },
      TemplateElement(node) {
        const kind = literalKind(node.value.raw);
        if (kind) check(node, kind, node.value.raw);
      },
      MemberExpression(node) {
        if (node.computed && isRampIndex(node)) {
          check(node, "ramp", sourceCode.getText(node));
        }
      },
    };
  },
};
