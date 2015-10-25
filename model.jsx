
// Asteroid dependencies -- asteroid doesn't do CommonJS so good.
import DDP from "ddp.js";
import Q from "q";

if (typeof window !== 'undefined') {
  window.DDP = DDP;
  window.Q = Q;
}

// Cool, now we can actually import you
import asteroid from "./node_modules/asteroid/dist/asteroid.browser.js";

const bellamie = new asteroid('drumstick.iame.li:3000');

class Model {
  constructor (collectionName) {
    bellamie.subscribe(collectionName);
    this.collection = bellamie.getCollection(collectionName);
  }

  get (selector = {}, cb) {
    const query = this.collection.reactiveQuery(selector);
    query.on('change', function(data) {
      console.log('change!', arguments);
      console.log('current state', JSON.stringify(query.result));
      window.result = query.result;
      cb(query.result);
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
