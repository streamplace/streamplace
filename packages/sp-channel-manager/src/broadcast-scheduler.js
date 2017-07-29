import Api from "kubernetes-client";
import SP from "sp-client";

export default class BroadcastScheduler {
  constructor() {
    // this.core = new Api.Core(Api.config.getInCluster());
    // SP.broadcasts.find().then(broadcasts => {
    //   console.log(broadcasts);
    // });

    SP.broadcasts.watch({}).on("data", broadcasts => {
      // console.log(broadcasts);
      // console.log(`${broadcast.id} changed or something`);
    });
  }
}
