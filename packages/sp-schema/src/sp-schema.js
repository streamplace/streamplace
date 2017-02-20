
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
    data.items.filter((item) => {
      return item.metadata.annotations &&
        item.metadata.annotations[SP_SCHEMA_ANNOTATION] === "true";
    })
    .forEach((configMap) => {
      Object.keys(configMap.data).forEach((name) => {
        const yaml = configMap.data[name];
        const plugin = configMap.metadata.annotations[SP_PLUGIN_ANNOTATION];
        newSchema.add({plugin, name, yaml});
      });
    });
    currentSchema = newSchema.get();
  })
  .catch((err) => {
    winston.error(err);
    throw err;
  });
};

app.get("/schema.json", function(req, res) {
  res.json(currentSchema);
});

getSchema().then(() => {
  const port = process.env.PORT || 80;
  winston.info(`sk-schema listening on port ${port}`);
  app.listen(port);
  setInterval(getSchema, 10000);
});

process.on("SIGTERM", function () {
  process.exit(0);
});
