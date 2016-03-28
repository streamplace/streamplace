
import winston from "winston";

// Configurables here
const ENV = {};
ENV.BELLAMIE_SERVER = process.env.BELLAMIE_SERVER;

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
