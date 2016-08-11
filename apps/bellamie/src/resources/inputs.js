
import Resource from "../resource";
import {randomID} from "../random";

export default class Input extends Resource {
  constructor() {
    super("inputs");
  }

  beforeCreate(newDoc) {
    return super.beforeCreate(newDoc).then((newDoc) => {
      newDoc.streamKey = randomID();
      newDoc.overlayKey = randomID();
      return newDoc;
    });
  }
}
