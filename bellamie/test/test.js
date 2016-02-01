
import SKClient from "sk-client";
import expect from "expect.js";

describe("broadcasts", function() {
  const SK = new SKClient({server: "http://localhost:80"});

  let testBroadcastId;
  let testBroadcastName = "Test Broadcast";

  const fail = function(err) {
    if (err instanceof Error) {
      throw err;
    }
    else {
      throw new Error(err.message);
    }
  };

  it("should create", function() {
    return SK.broadcasts.create({name: testBroadcastName}).then(function(broadcast) {
      expect(broadcast).to.be.ok();
      expect(broadcast.name).to.be(testBroadcastName);
      testBroadcastId = broadcast.id;
    })
    .catch(fail);
  });

  it("should findOne", function() {
    return SK.broadcasts.findOne(testBroadcastId).then(function(broadcast) {
      expect(broadcast).to.be.ok();
      expect(broadcast.name).to.be(testBroadcastName);
    })
    .catch(fail);
  });

  it("should update", function() {
    testBroadcastName = "Altered Name";
    return SK.broadcasts.update(testBroadcastId, {name: testBroadcastName})
    .then(function(broadcast) {
      expect(broadcast).to.be.ok();
      expect(broadcast.name).to.be(testBroadcastName);
    })
    .catch(fail);
  });

  it("should find", function() {
    return SK.broadcasts.find().then(function(broadcasts) {
      expect(broadcasts).to.be.an(Array);
      expect(broadcasts[0]).to.be.ok();
      expect(broadcasts[0].name).to.be(testBroadcastName);
    })
    .catch(fail);
  });

  it("should delete", function() {
    return SK.broadcasts.delete(testBroadcastId)
    .then(function() {
      expect(true).to.be(false);
      return SK.broadcasts.findOne(testBroadcastId).then(function() {
        throw new Error("Uh oh -- success on findOne after delete");
      })
      .catch(function(err) {
        expect(err.status).to.be(404);
      });
    })
    .catch(fail);
  });
});
