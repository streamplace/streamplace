export interface SlashCommandResult {
  handled: boolean;
  error?: string;
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

const commands = new Map<string, SlashCommand>();

export function registerSlashCommand(command: SlashCommand) {
  commands.set(command.name, command);
}

export function unregisterSlashCommand(name: string) {
  commands.delete(name);
}

export async function handleSlashCommand(
  input: string,
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

  const command = commands.get(commandName);
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

export function getRegisteredCommands(): SlashCommand[] {
  return Array.from(commands.values());
}
