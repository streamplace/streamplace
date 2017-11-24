import Api from "kubernetes-client";
import SP from "sp-client";
import winston from "winston";
import config from "sp-configuration";

// Kinda messy. Improvements to be made here:
// * Abstract it so it can work with podtemplates so it can get used for
//   compositors as well as broadcasters
// * When shutting down a pod, detect the "Terminating" state and don't just
//   delete it over and over you rube
// * Maybe use Jobs instead of Pods so they can self-destruct by exiting with
//   code 0 when they become inactive?

const BROADCASTER_IMAGE = config.require("BROADCASTER_IMAGE");
const IMAGE_PULL_POLICY = config.require("IMAGE_PULL_POLICY");
const DEV_ROOT_DIRECTORY = config.optional("DEV_ROOT_DIRECTORY");
const IMAGE_PULL_SECRETS = config.optional("IMAGE_PULL_SECRETS");
let imagePullSecrets = [];
if (IMAGE_PULL_SECRETS) {
  imagePullSecrets = JSON.parse(IMAGE_PULL_SECRETS);
}

const ROLE_ANNOTATION = "stream.place/role";
const MY_ROLE = "broadcaster";
const BROADCAST_ID_ANNOTATION = "stream.place/broadcast-id";

export default class BroadcastScheduler {
  constructor() {
    this.core = new Api.Core(Api.config.getInCluster());
    // SP.broadcasts.find().then(broadcasts => {
    //   console.log(broadcasts);
    // });

    SP.broadcasts
      .watch({ active: true })
      .on("data", broadcasts => (this.broadcasts = broadcasts))
      .on("newDoc", broadcast => this.reconcileAdded(broadcast))
      .on("deletedDoc", id => this.reconcileDeleted(id))
      .then(broadcasts => {
        this.broadcasts = broadcasts;
        setInterval(() => {
          this.periodicReconcile();
        }, 5000);
      });
  }

  periodicReconcile() {
    this.broadcasts.forEach(broadcast => {
      this.reconcileAdded(broadcast);
    });
    this.getPods().then(pods => {
      const broadcastIds = this.broadcasts.map(broadcast => broadcast.id);
      const podIds = [];
      pods.forEach(pod => {
        // Only delete once for each podId
        if (!pod.metadata.annotations) {
          winston.info(`${pod.name} has no annotations, that's weird`);
          return;
        }
        const podId = pod.metadata.annotations[BROADCAST_ID_ANNOTATION];
        if (podIds.includes(podId)) {
          return;
        }
        podIds.push(podId);
        if (!broadcastIds.includes(podId)) {
          winston.info(
            `Found orphaned broadcaster ${pod.metadata.name}, deleting`
          );
          this.reconcileDeleted(podId);
        }
      });
    });
  }

  getPods() {
    return new Promise((resolve, reject) => {
      this.core.namespaces.pods.get((err, data) => {
        if (err) {
          return reject(err);
        }
        data = data.items.filter(pod => {
          const annotations = pod.metadata.annotations;
          return annotations && annotations[ROLE_ANNOTATION] === MY_ROLE;
        });
        resolve(data);
      });
    });
  }

  reconcileAdded(broadcast) {
    winston.debug(`Reconciling broadcast ${broadcast.id}`);
    this.getPodsForBroadcast(broadcast)
      .then(([currentPod]) => {
        if (currentPod) {
          winston.debug(
            `Broadcast ${broadcast.id} has pod ${
              currentPod.metadata.name
            }, we're good`
          );
          return;
        }
        return this.createPod(broadcast);
      })
      .catch(err => {
        winston.error(`Error reconciling broadcast ${broadcast.id}`);
        winston.error(err);
      });
  }

  reconcileDeleted(id) {
    return this.getPodsForBroadcast(id)
      .then(pods => {
        pods.forEach(pod => {
          winston.info(`Deleting pod ${pod.metadata.name}`);
          return this.deletePod(pod);
        });
      })
      .catch(err => {
        winston.error(`Error deleting broadcast ${id}`);
        winston.error(err);
      });
  }

  getPodsForBroadcast(broadcastId) {
    if (typeof broadcastId !== "string") {
      broadcastId = broadcastId.id;
    }
    return this.getPods().then(pods => {
      const [currentPod, ...rest] = pods.filter(pod => {
        const annotations = pod.metadata.annotations;
        return (
          annotations && annotations[BROADCAST_ID_ANNOTATION] === broadcastId
        );
      });
      if (rest.length > 0) {
        winston.warn(
          `${rest.length} pods found for broadcast ${broadcastId}, wtf`
        );
      }
      return [currentPod, ...rest];
    });
  }

  generatePod(broadcast) {
    const podData = {
      apiVersion: "v1",
      metadata: {
        generateName: `broadcaster-${broadcast.id.slice(0, 8)}-`,
        annotations: {
          [ROLE_ANNOTATION]: MY_ROLE,
          [BROADCAST_ID_ANNOTATION]: broadcast.id
        }
      },
      spec: {
        imagePullSecrets: imagePullSecrets,
        terminationGracePeriodSeconds: 5,
        containers: [
          {
            name: "broadcaster",
            image: BROADCASTER_IMAGE,
            imagePullPolicy: IMAGE_PULL_POLICY,
            // Copy over all of our own SP_ environment variables
            env: Object.keys(process.env)
              .filter(key => key.slice(0, 3) === "SP_")
              .map(name => {
                const value = process.env[name];
                return { name, value };
              })
              .concat([
                {
                  name: "SP_BROADCAST_ID",
                  value: broadcast.id
                },
                {
                  name: "DEBUG",
                  value: "sp:*"
                },
                {
                  name: "SP_POD_IP",
                  valueFrom: {
                    fieldRef: {
                      fieldPath: "status.podIP"
                    }
                  }
                }
              ])
          }
        ]
      }
    };
    // If we're in development, add a bunch of annoying crap
    if (DEV_ROOT_DIRECTORY) {
      podData.spec.containers[0].command = [
        "bash",
        "-c",
        `TMPDIR="/tmp" exec /app/node_modules/.bin/babel-watch -o '/app/src/*' /app/src/sp-broadcaster.js`
      ];
      podData.spec.containers[0].volumeMounts = [
        {
          name: "app",
          mountPath: "/app"
        },
        {
          name: "streamplace",
          mountPath: DEV_ROOT_DIRECTORY
        },
        {
          name: "tmp",
          mountPath: "/tmp"
        }
      ];
      podData.spec.volumes = [
        {
          name: "streamplace",
          hostPath: { path: DEV_ROOT_DIRECTORY }
        },
        {
          name: "app",
          hostPath: { path: `${DEV_ROOT_DIRECTORY}/packages/sp-broadcaster` }
        },
        {
          name: "tmp",
          emptyDir: {}
        }
      ];
    }
    return podData;
  }

  createPod(broadcast) {
    winston.debug(`creating pod for ${broadcast.id}`);
    return new Promise((resolve, reject) => {
      this.core.namespaces.pods.post(
        {
          body: this.generatePod(broadcast)
        },
        (err, newPod) => {
          if (err) {
            return reject(err);
          }
          resolve(newPod);
        }
      );
    }).then(newPod => {
      winston.info(`Created pod ${newPod.metadata.name}`);
    });
  }

  deletePod(pod) {
    return new Promise((resolve, reject) => {
      this.core.namespaces.pods.delete(pod.metadata.name, err => {
        if (err) {
          reject(err);
        }
        resolve();
      });
    });
  }
}
