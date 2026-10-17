import type { TeleportErrorData } from "./slash-commands/teleport";

export interface SlashCommandResult {
  handled: boolean;
  error?: string;
  errorData?: TeleportErrorData;
}

export type SlashCommandHandler = (
  args: string[],
  rawInput: string,
) => Promise<SlashCommandResult>;

export interface SlashCommand {
  name: string;
  description: string;
  usage: string;
  handler: SlashCommandHandler;
}

export async function handleSlashCommand(
  input: string,
  commands: SlashCommand[],
): Promise<SlashCommandResult> {
  const trimmed = input.trim();
  if (!trimmed.startsWith("/")) {
    return { handled: false };
  }

  const parts = trimmed.slice(1).split(/\s+/);
  const commandName = parts[0]?.toLowerCase();
  const args = parts.slice(1);

  if (!commandName) {
    return { handled: false };
  }

  const command = commands.find((command) => command.name === commandName);
  if (!command) {
    return {
      // for now - return false
      handled: false,
      error: `Unknown command: /${commandName}`,
    };
  }

  try {
    return await command.handler(args, trimmed);
  } catch (err) {
    return {
      handled: true,
      error: err instanceof Error ? err.message : "Command failed",
    };
  }
}
