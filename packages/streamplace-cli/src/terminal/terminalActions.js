
import {
  TERMINAL_COMMAND
} from "../constants/actionNames";

export function terminalCommand(command) {
  return {
    type: TERMINAL_COMMAND,
    command: command
  };
}
