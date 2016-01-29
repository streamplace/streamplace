
import winston from "winston";

// Configurables here
const ENV = {};
ENV.PORT = process.env.PORT;
ENV.RETHINK_HOST = process.env.RETHINK_HOST;
ENV.RETHINK_PORT = process.env.RETHINK_PORT;
ENV.RETHINK_DATABASE = process.env.RETHINK_DATABASE;

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
