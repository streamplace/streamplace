import { generatePrivateKey, privateKeyToAccount } from "viem/accounts";

const key = generatePrivateKey();
console.log(`AQD_ADMIN_ACCOUNT_KEY=${key}`);
const account = privateKeyToAccount(key);
console.log(`AQ_ADMIN_ACCOUNT=${account.address.toLowerCase()}`);
