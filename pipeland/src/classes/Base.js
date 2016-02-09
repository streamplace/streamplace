
// What?! A base class in Javascript?! Look man, Pipeland is polymorphic as hell. All different
// Vertex interfaces extending each other and shit. We got to have a base class just to make sense
// of any of this garbage.

import winston from "winston";

export default class Base {
  constructor() {

  }

  info(first, ...args) {
    if (typeof first !== "string") {
      args = [first, ...args];
      first = "";
    }
    winston.info(`[${this.constructor.name} ${this.id}] ${first}`, ...args);
  }

  error(first, ...args) {
    if (typeof first !== "string") {
      args = [first, ...args];
      first = "";
    }
    winston.error(`[${this.constructor.name} ${this.id}] ${first}`, ...args);
  }
}
