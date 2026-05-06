import { Agent } from "@atproto/api";
import { schemas as parentSchemas } from "@atproto/api/dist/client/lexicons";
import { SessionManager } from "@atproto/api/dist/session-manager";
import { Lexicons } from "@atproto/lexicon";
import { PlaceNS } from "./lexicons";
import { schemas as appSchemas } from "./lexicons/lexicons";

export class StreamplaceAgent extends Agent {
  place = new PlaceNS(this);
  lex: Lexicons;

  constructor(options: string | URL | SessionManager) {
    super(options);

    const streamplaceSchemas = appSchemas.filter((x) =>
      x.id.startsWith("place.stream"),
    );

    // for game search
    const pentaractSchemas = appSchemas.filter((x) =>
      x.id.startsWith("games.gamesgamesgamesgames"),
    );

    this.lex = new Lexicons([
      ...parentSchemas,
      ...streamplaceSchemas,
      ...pentaractSchemas,
    ]);
  }
}
