import { Agent } from "@atproto/api";
import { Client } from "@atproto/lex";

export class StreamplaceAgent extends Agent {
  // The @atproto/lex client for Streamplace (place.stream.* / games.*) XRPC and
  // records. It rides on the same session/fetchHandler as the @atproto/api Agent,
  // which we keep for OAuth and Bluesky (app.bsky.*) reads.
  client: Client;

  constructor(options: ConstructorParameters<typeof Agent>[0]) {
    super(options);

    this.client = new Client({
      did: this.did as `did:${string}:${string}` | undefined,
      fetchHandler: this.fetchHandler,
    });
  }
}
