import { Secp256k1Keypair, bytesToMultibase } from "@atproto/crypto";

const keypair = await Secp256k1Keypair.create({ exportable: true });
const exportedKey = await keypair.export();
const multibaseKey = bytesToMultibase(exportedKey, "base58btc");
console.log(keypair.did());
console.log(multibaseKey);
