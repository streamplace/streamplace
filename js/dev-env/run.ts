import { TestNetwork } from "./lib";

(async () => {
  const network = await TestNetwork.create({});
  console.log("hi");
})();
