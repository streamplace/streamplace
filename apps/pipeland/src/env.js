
/**
 * TODO: This file isn't necessary anymore, need to go through Pipeland and change all these
 * references.
 */

import winston from "winston";
import config from "sk-config";

// Required variables.
const ENV = {};
ENV.API_SERVER_URL = config.require("API_SERVER_URL");
ENV.RTMP_URL_INTERNAL = config.require("RTMP_URL_INTERNAL");
ENV.RTMP_URL_EXTERNAL = config.require("PUBLIC_RTMP_URL_PREVIEW");

// Optional Variables.
ENV.AWS_ACCESS_KEY_ID = config.require("AWS_ACCESS_KEY_ID");
ENV.AWS_SECRET_ACCESS_KEY = config.require("AWS_SECRET_ACCESS_KEY");
ENV.AWS_USER_UPLOAD_BUCKET = config.require("AWS_USER_UPLOAD_BUCKET");
ENV.AWS_USER_UPLOAD_PREFIX = config.require("AWS_USER_UPLOAD_PREFIX");
ENV.AWS_USER_UPLOAD_REGION = config.require("AWS_USER_UPLOAD_REGION");

// #crash-on-start-hack
ENV.CRASH_ON_BROADCAST_START = process.env.CRASH_ON_BROADCAST_START;

export default ENV;
