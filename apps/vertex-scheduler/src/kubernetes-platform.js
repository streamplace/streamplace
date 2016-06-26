
import fs from "fs";
import request from "request-promise";
import config from "sk-config";
import winston from "winston";
import {parse} from "url";

// Currently this just supports auth through TLS keys 'cause that seems like the gold standard for
// Kubernetes.

const isKube = config.require("PLATFORM") === "kubernetes";
let KUBE_HOST = isKube && config.require("KUBE_HOST");
let KUBE_CA_FILE = isKube && config.require("KUBE_CA_FILE");
let KUBE_TOKEN_FILE = isKube && config.require("KUBE_TOKEN_FILE");
let KUBE_NAMESPACE = isKube && config.require("KUBE_NAMESPACE");

export default class KubernetesPlatform {
  constructor() {
    this.url = parse(KUBE_HOST).href; // Easy way to normalize trailing slashes
    this.kubeToken = fs.readFileSync(KUBE_TOKEN_FILE, "utf8");
    this.kubeCA = fs.readFileSync(KUBE_CA_FILE, "utf8");
  }

  _http(method, url, data) {
    const params = {
      method: method,
      url: `${this.url}api/v1/namespaces/${KUBE_NAMESPACE}${url}`,
      ca: this.kubeCA,
      headers: {
        "Authorization": `Bearer ${this.kubeToken}`
      },
      json: true,
    };
    if (data) {
      params.body = data;
    }
    return request(params)
    .then((data) => {
      return data;
    })
    .catch((err) => {
      winston.error("Error connecting to Kubernetes server: ", err);
      throw err;
    });
  }

  _generateJob(vertex, environment) {
    return {
      apiVersion: "v1",
      kind: "Pod",
      metadata: {
        generateName: `vertex-${vertex.type.toLowerCase()}-`,
        labels: {
          "kitchen.stream.vertex-processor": "true",
          "kitchen.stream.vertex-id": vertex.id,
        },
      },
      spec: {
        restartPolicy: "OnFailure",
        terminationGracePeriodSeconds: 5,
        containers: [{
          name: "pipeland",
          image: vertex.image,
          env: Object.keys(environment).map((key) => {
            return {
              name: `SK_${key}`,
              value: `${environment[key]}`,
            };
          }),
        }]
      }
    };
  }

  getVertexProcessors() {
    return this._http("GET", "/pods").then((podList) => {
      return podList.items
      .filter((pod) => {
        return pod.metadata.labels["kitchen.stream.vertex-processor"] === "true" &&
          !pod.metadata.deletionTimestamp; // Ignore if we already deleted and it's spinning down
      })
      .map((pod) => {
        return {
          id: pod.metadata.name,
          vertexId: pod.metadata.labels["kitchen.stream.vertex-id"]
        };
      });
    });
  }

  createVertexProcessor(vertex, environment) {
    return this._http("POST", "/pods", this._generateJob(vertex, environment))
    .then((job, test) => {
      return {
        id: job.metadata.name,
        vertexId: job.metadata.labels["kitchen.stream.vertex-id"],
      };
    });
  }

  removeVertexProcessor(id) {
    return this._http("DELETE", `/pods/${id}`);
  }
}
