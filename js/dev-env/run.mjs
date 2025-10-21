import { TestNetwork } from "./dist/index.js";

(async () => {
  const network = await TestNetwork.create({
    pds: {
      publicUrl: process.env.TEST_ENV_PDS_PUBLIC_URL,
      port: process.env.TEST_ENV_PDS_PORT,
      hostname: process.env.TEST_ENV_PDS_HOSTNAME,
    },
    plc: {
      port: process.env.TEST_ENV_PLC_PORT,
    },
  });
  console.log(
    JSON.stringify({ "pds-url": network.pds.url, "plc-url": network.plc.url }),
  );
})();
