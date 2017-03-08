
import express from "express";
import winston from "winston";
import {exec} from "mz/child_process";
import {Schema} from "./schema-class";

const SP_SCHEMA_ANNOTATION = "stream.place/sp-schema";
const SP_PLUGIN_ANNOTATION = "stream.place/sp-plugin";

const app = express();

let currentSchema = (new Schema()).get();

const getSchema = function() {
  return exec("kubectl get configmap -o json").then(([stdout, stderr]) => {
    const data = JSON.parse(stdout);
    const newSchema = new Schema();
    data.items.filter((configMap) => {
      return configMap.metadata.annotations &&
        configMap.metadata.annotations[SP_PLUGIN_ANNOTATION] !== undefined;
    })
    .forEach((configMap) => {
      if (configMap.metadata.annotations[SP_SCHEMA_ANNOTATION] === "true") {
        Object.keys(configMap.data).forEach((name) => {
          const yaml = configMap.data[name];
          const plugin = configMap.metadata.annotations[SP_PLUGIN_ANNOTATION];
          newSchema.addSchema({plugin, name, yaml});
        });
      }
      else {
        const plugin = configMap.metadata.annotations[SP_PLUGIN_ANNOTATION];
        newSchema.addConfiguration({plugin, data: configMap.data});
      }
    });
    currentSchema = newSchema.get();
  })
  .catch((err) => {
    winston.error(err);
    throw err;
  });
};

app.options("*", function(req, res) {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Methods", "GET");
});

app.get("/schema.json", function(req, res) {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Methods", "GET");
  res.json(currentSchema);
});

getSchema().then(() => {
  const port = process.env.PORT || 80;
  winston.info(`sp-schema listening on port ${port}`);
  app.listen(port);
  setInterval(getSchema, 10000);
});

process.on("SIGTERM", function () {
  process.exit(0);
});
