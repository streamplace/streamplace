
import winston from "winston";

// Configurables here
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

export default ENV;
