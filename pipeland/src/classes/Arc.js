
import Base from "./Base";
import SK from "../sk";

export default class Arc extends Base {
  constructor(params) {
    super(params);
    // Watch my vertex, so I can respond appropriately.
    const {id, broadcast} = params;
    this.id = id;
    this.broadcast = broadcast;
    SK.arcs.watch({id: this.id})

    .then((docs) => {
      this.doc = docs[0];
      this.info("Got initial pull.");
      this.init();
    })

    .catch((err) => {
      this.error("Error on initial pull", err);
    });
  }

  init() {
    const from = this.doc.from;
    const to = this.doc.to;
    this.info(`Arcing vertex ${from.vertexId}:${from.pipe} to vertex ${to.vertexId}:${to.pipe}`);
    const fromVertex = this.broadcast.getVertex(from.vertexId);
    const toVertex = this.broadcast.getVertex(to.vertexId);
    fromVertex.getPipe(from.pipe).pipe(toVertex.getPipe(to.pipe));
  }
}

Arc.create = function(params) {
  return new Arc(params);
};
