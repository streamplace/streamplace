import { formatMultikey } from "@atproto/crypto";
import { AppContext, Database, PlcServer } from "@did-plc/server";
import type { ServerResponse } from "node:http";
import * as ui8 from "uint8arrays";

import getPort from "get-port";

export interface PlcServerOptions {
  port?: number;
}

export class TestPlcServer {
  constructor(
    public readonly server: PlcServer,
    public readonly url: string,
    public readonly port: number,
  ) {}

  static async create(cfg: PlcServerOptions = {}): Promise<TestPlcServer> {
    const port = cfg.port ?? (await getPort());
    const url = `http://localhost:${port}`;

    const db = Database.mock();
    const server = PlcServer.create({ db, port });

    // The fixture PLC predates Multikey. Modernize its DID documents while
    // preserving the public keys and the signed operation log.
    type Response = ServerResponse & { send(body?: unknown): unknown };
    const { response } = server.app as unknown as { response: Response };
    const send = response.send;
    response.send = function (this: Response, body?: unknown) {
      const contentType = this.getHeader("content-type");
      if (
        typeof body === "string" &&
        typeof contentType === "string" &&
        contentType.startsWith("application/did+ld+json")
      ) {
        const doc = JSON.parse(body);
        for (const method of doc.verificationMethod) {
          const alg =
            method.type === "EcdsaSecp256k1VerificationKey2019"
              ? "ES256K"
              : method.type === "EcdsaSecp256r1VerificationKey2019"
                ? "ES256"
                : null;
          if (!alg) continue;
          method.type = "Multikey";
          method.publicKeyMultibase = formatMultikey(
            alg,
            ui8.fromString(method.publicKeyMultibase.slice(1), "base58btc"),
          );
        }
        doc["@context"].push("https://w3id.org/security/multikey/v1");
        body = JSON.stringify(doc);
      }
      return send.call(this, body);
    };

    await server.start();

    return new TestPlcServer(server, url, port);
  }

  get context(): AppContext {
    return this.server.ctx;
  }

  async close() {
    await this.server.destroy();
  }
}
