// Export primitive components
export * from "./primitives/button";
export * from "./primitives/input";
export * from "./primitives/modal";
export * from "./primitives/text";

// Export styled components
export * from "./button";
export * from "./dialog";
export * from "./dropdown";
export * from "./icons";
export * from "./input";
export * from "./loader";
export * from "./resizeable";
export * from "./slider";
export * from "./text";
export * from "./toast";
export * from "./view";

// Component collections for easy importing
export { ButtonPrimitive } from "./primitives/button";
export { InputPrimitive } from "./primitives/input";
export { ModalPrimitive } from "./primitives/modal";
export { TextPrimitive } from "./primitives/text";

// Re-export commonly used types
export type { Theme } from "../../lib/theme/theme";
export type { ButtonProps } from "./button";
export type { DialogProps } from "./dialog";
export type { InputProps } from "./input";
export type { TextProps } from "./text";
export type { ViewProps } from "./view";

export * from "../../lib/theme";
