import { ExplanationDialog } from "./explanation-dialog";

export interface WarningHelpProps {
  warning: { label: string; description: string };
}

export function WarningHelp({ warning }: WarningHelpProps) {
  return (
    <ExplanationDialog
      compact
      title={`About ${warning.label}`}
      entries={[warning]}
    />
  );
}
