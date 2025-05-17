import { Agent } from "@atproto/api";
import { SessionManager } from "@atproto/api/dist/session-manager";
import { PlaceNS } from "lexicons";
import { schemas as parentSchemas } from "@atproto/api/dist/client/lexicons";
import { schemas as appSchemas } from "../../lexicons/lexicons";
import { Lexicons } from "@atproto/lexicon";
export class StreamplaceAgent extends Agent {
  place = new PlaceNS(this);
  lex: Lexicons;

  constructor(options: string | URL | SessionManager) {
    super(options);

    const streamplaceSchemas = appSchemas.filter((x) =>
      x.id.startsWith("place.stream"),
    );

    this.lex = new Lexicons([...parentSchemas, ...streamplaceSchemas]);
  }
}
