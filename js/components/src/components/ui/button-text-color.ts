import { createContext } from "react";

/**
 * Set by <Button> to its variant's text color, consumed by <Text> as the
 * default color when a caller didn't pass one. Lets element children (e.g.
 * `<Button><Text>Save</Text></Button>`) follow the button's foreground —
 * essential now that the primary fill is Paper/Ink, where an un-inherited
 * near-white default would be invisible. Undefined outside a button.
 */
export const ButtonTextColorContext = createContext<string | undefined>(
  undefined,
);
