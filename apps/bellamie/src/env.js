
import winston from "winston";
import config from "sk-config";

// Configurables here
const ENV = {};
ENV.RETHINK_HOST = config.require("RETHINK_HOST");
ENV.RETHINK_PORT = config.require("RETHINK_PORT");
ENV.RETHINK_DATABASE = config.require("RETHINK_DATABASE");
ENV.PORT = config.require("BELLAMIE_PORT");

export default ENV;
