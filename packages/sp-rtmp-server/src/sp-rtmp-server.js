import express from "express";

const app = express();

app.get("/healthz", function(req, res, next) {
  res.sendStatus(200);
});

app.listen(80);
