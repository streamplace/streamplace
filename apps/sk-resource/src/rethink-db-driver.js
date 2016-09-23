
const req = function() {
  throw new Error("Missing required parameter");
};

export default class RethinkDbDriver {
  constructor() {

  }

  find(ctx = req(), query = req()) {

  }

  findOne(ctx = req(), id = req()) {

  }

  upsert(ctx = req(), doc = req()) {

  }

  delete(ctx = req(), id = req()) {

  }

  watch(ctx = req(), query = req()) {

  }
}
