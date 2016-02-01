
// "Base class" from which the individual resources "inherit". I mean. That's not how it actually 
// works, but think about it that way.

import r from "rethinkdb";

export default class Resource {

  index(req, res, next) {
    res.status(501);
    res.json({
      code: "NOT_IMPLEMENTED",
      message: "This thing doesn't do nothing yet."
    });
    next();
  }

  get(req, res, next) {
    res.status(501);
    res.json({
      code: "NOT_IMPLEMENTED",
      message: "This thing doesn't do nothing yet."
    });
    next();
  }

  post(req, res, next) {
    res.status(501);
    res.json({
      code: "NOT_IMPLEMENTED",
      message: "This thing doesn't do nothing yet."
    });
    next();
  }

  put(req, res, next) {
    res.status(501);
    res.json({
      code: "NOT_IMPLEMENTED",
      message: "This thing doesn't do nothing yet."
    });
    next();
  }

  delete(req, res, next) {
    res.status(501);
    res.json({
      code: "NOT_IMPLEMENTED",
      message: "This thing doesn't do nothing yet."
    });
    next();
  }

  /**
   * Real simple. Just takes an object of properties. This lets us "subclass" real easy.
   */
  constructor(props) {
    Object.keys(props).forEach((key) => {
      this[key] = props[key];
    });
  }

  /**
   * Make sure they're allowed to do the thing that they're doing.
   * @param  {[type]}   req  [description]
   * @param  {[type]}   res  [description]
   * @param  {Function} next [description]
   * @return {[type]}        [description]
   */
  auth (req, res, next) {
    next();
  }

  validate (req, res, next) {
    next();
  }

  beforeSave (req, res, next) {
    next();
  }

  save (req, res, next) {
    next();
  }

  postSave (req, res, next) {
    next();
  }
}
