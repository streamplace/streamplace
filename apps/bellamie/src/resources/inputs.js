
import Resource from "../resource";
import {randomID} from "../random";

export default class Input extends Resource {
  constructor() {
    super();
    this.name = "inputs";
  }

  beforeCreate(newDoc) {
    return super.beforeCreate(newDoc).then((newDoc) => {
      newDoc.streamKey = randomID();
      return newDoc;
    });
  }
}
