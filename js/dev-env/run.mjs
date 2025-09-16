import { TestNetwork } from "./dist/index.js";

(async () => {
  const network = await TestNetwork.create({});
  console.log(
    JSON.stringify({ "pds-url": network.pds.url, "plc-url": network.plc.url }),
  );
})();
