import admin from "firebase-admin";
import fs from "fs";

const serviceAccount = JSON.parse(
  fs.readFileSync(`${process.env.AQKEYS}/firebase-admin.json`, "utf8"),
);

admin.initializeApp({
  credential: admin.credential.cert(serviceAccount),
});

// const tokens = JSON.parse(process.argv.slice(2))
// console.log(tokens)
// process.exit(0)

// curl -H "Authorization: Bearer $SP_ADMIN" https://stream.place/api/notification

const delay = (ms) => new Promise((r) => setTimeout(r, ms));
const blast = async (body) => {
  const tokensRes = await fetch("https://stream.place/api/notification", {
    headers: {
      Authorization: `Bearer ${process.env.SP_ADMIN}`,
    },
  });
  const fullTokens = await tokensRes.json();
  const tokens = fullTokens.map((t) => t.Token);
  const notification = {
    tokens: tokens,
    notification: {
      title: "🔴 @iame.li is LIVE!",
      body: body,
      // imageUrl: "https://my-cdn.com/app-logo.png",
    },
    apns: {
      headers: {
        "apns-priority": "10",
      },
      payload: {
        aps: {
          sound: "default",
        },
      },
    },
    android: {
      priority: "high",
      notification: {
        sound: "default",
      },
    },
    priority: "high",
  };
  console.log(JSON.stringify(notification.notification, null, 2));
  // console.log("launching in 5 sec")
  // await delay(5000)
  const res = await admin.messaging().sendEachForMulticast(notification);
  console.log(JSON.stringify(res));
};
export default blast;

import { resolve } from "path";
import { fileURLToPath } from "url";

const pathToThisFile = resolve(fileURLToPath(import.meta.url));
const pathPassedToNode = resolve(process.argv[1]);
const isThisFileBeingRunViaCLI = pathToThisFile.includes(pathPassedToNode);
if (isThisFileBeingRunViaCLI) {
  blast(process.argv[2]);
}
