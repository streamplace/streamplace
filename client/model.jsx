
if (typeof window !== 'undefined') {
  // browser.
  import DDP from "ddp.js";
  import Q from "q";
  window.DDP = DDP;
  window.Q = Q;
  import asteroid from "asteroid/dist/asteroid.browser.js";
}
else {
  // node. cool.
  import asteroid from "asteroid";
}

const bellamie = new asteroid('drumstick.iame.li:3000');

class Model {
  constructor (collectionName) {
    bellamie.subscribe(collectionName);
    this.collection = bellamie.getCollection(collectionName);
  }

  get (selector = {}, cb) {
    const query = this.collection.reactiveQuery(selector);
    // This fires a few times per cycle. Let's keep a bit to stop that
    let firing = false;
    query.on('change', function(data) {
      if (!firing) {
        firing = true;
        setTimeout(function() {
          firing = false;
          cb(query.result);
        }, 0);
      }
    })
    cb(query.result);
  }

  insert (...args) {
    return this.collection.insert(...args);
  }

  update (...args) {
    return this.collection.update(...args);
  }

  remove (...args) {
    return this.collection.remove(...args);
  }
}

export const Broadcast = new Model('broadcasts');
