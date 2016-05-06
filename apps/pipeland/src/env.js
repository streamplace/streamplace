
import winston from "winston";

// Required variables.
const ENV = {};
ENV.API_SERVER_URL = process.env.API_SERVER_URL;

let exit = false;
Object.keys(ENV).forEach((key) => {
  if (!ENV[key]) {
    winston.error(`Missing required environment variable: ${key}`);
    exit = true;
  }
});
if (exit) {
  process.exit(1);
}

// Optional Variables.
ENV.AWS_ACCESS_KEY_ID = process.env.AWS_ACCESS_KEY_ID;
ENV.AWS_SECRET_ACCESS_KEY = process.env.AWS_SECRET_ACCESS_KEY;
ENV.AWS_USER_UPLOAD_BUCKET = process.env.AWS_USER_UPLOAD_BUCKET;
ENV.AWS_USER_UPLOAD_PREFIX = process.env.AWS_USER_UPLOAD_PREFIX;
ENV.AWS_USER_UPLOAD_REGION = process.env.AWS_USER_UPLOAD_REGION;

export default ENV;
