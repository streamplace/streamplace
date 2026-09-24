// Disallows hardcoded style literals in component code: raw hex colors,
// rgb()/rgba() strings, and raw palette-ramp indexing (colors.primary[500]).

const RAMPS = new Set(
  "slate gray zinc neutral stone red orange amber yellow lime green emerald teal cyan sky blue indigo violet purple fuchsia pink rose primary destructive success warning".split(
    " ",
  ),
);

const HEX = /#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})\b/;
const RGB = /\brgba?\(/;

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
        'Hardcoded {{kind}} style literal "{{text}}". Read the value from the theme, or suppress the line with // eslint-disable-line streamplace/no-token-literals.',
    },
  },
  create(context) {
    const sourceCode = context.sourceCode ?? context.getSourceCode();
    const report = (node, kind, text) =>
      context.report({ node, messageId: "literal", data: { kind, text } });

    return {
      Literal(node) {
        const kind = literalKind(node.value);
        if (kind) report(node, kind, String(node.value));
      },
      TemplateElement(node) {
        const kind = literalKind(node.value.raw);
        if (kind) report(node, kind, node.value.raw);
      },
      MemberExpression(node) {
        if (node.computed && isRampIndex(node)) {
          // Point at the index expression, not the whole member chain: a
          // formatter can wrap `colors.primary[500]` across lines, and the
          // trailing eslint-disable-line comment lands on the index.
          context.report({
            node,
            loc: node.property.loc,
            messageId: "literal",
            data: { kind: "ramp", text: sourceCode.getText(node) },
          });
        }
      },
    };
  },
};
