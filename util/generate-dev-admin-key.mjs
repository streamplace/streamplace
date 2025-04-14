import { generatePrivateKey, privateKeyToAccount } from "viem/accounts";

const key = generatePrivateKey();
console.log(`AQD_ADMIN_ACCOUNT_KEY=${key}`);
const account = privateKeyToAccount(key);
console.log(`SP_ADMIN_ACCOUNT=${account.address.toLowerCase()}`);
