
import express from "express";
import winston from "winston";

const app = express();

const port = process.env.PORT || 80;
winston.info(`sk-schema listening on port ${port}`);
app.listen(port);
