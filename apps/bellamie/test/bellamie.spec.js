
import SKClient from "sk-client";
import expect from "expect.js";

describe("broadcasts", function() {
  const SK = new SKClient({
    server: "http://localhost:8100",
    log: true,
  });

  let testBroadcastId = "nope";
  let testBroadcastTitle = "Test Broadcast";

  const fail = function(err) {
    if (err instanceof Error) {
      throw err;
    }
    else {
      throw new Error(err.message);
    }
  };

  it("should create", function() {
    return SK.broadcasts.create({title: testBroadcastTitle}).then(function(broadcast) {
      expect(broadcast).to.be.ok();
      expect(broadcast.title).to.be(testBroadcastTitle);
      testBroadcastId = broadcast.id;
    })
    .catch(fail);
  });

  it("should findOne", function() {
    return SK.broadcasts.findOne(testBroadcastId).then(function(broadcast) {
      expect(broadcast).to.be.ok();
      expect(broadcast.title).to.be(testBroadcastTitle);
    })
    .catch(fail);
  });

  it("should update", function() {
    testBroadcastTitle = "Altered Name";
    return SK.broadcasts.update(testBroadcastId, {title: testBroadcastTitle})
    .then(function(broadcast) {
      expect(broadcast).to.be.ok();
      expect(broadcast.title).to.be(testBroadcastTitle);
    })
    .catch(fail);
  });

  it("should find", function() {
    return SK.broadcasts.find().then(function(broadcasts) {
      expect(broadcasts).to.be.an(Array);
      const myBroadcast = broadcasts.filter((b) => b.id === testBroadcastId)[0];
      expect(myBroadcast).to.be.ok();
      expect(myBroadcast.title).to.be(testBroadcastTitle);
    })
    .catch(fail);
  });

  describe("watch", function() {
    let data;
    let watchBroadcastTitle = "broadcast watch test";
    let watchBroadcastId;
    let broadcastCursor;

    // Given a cursor and an eventName, return a promise that has resolved after the event has
    // fired once.
    let eventPromise = function(eventName) {
      return new Promise((resolve, reject) => {
        broadcastCursor.on(eventName, (...args) => {
          if (resolve) {
            resolve(args);
            resolve = null;
          }
        });
      });
    };

    it("should initalize", function(done) {
      broadcastCursor = SK.broadcasts.watch().then(function() {
        done();
      });
      return broadcastCursor;
    });


    it("should recieve created events", function() {
      const createdEventPromise = eventPromise("created");
      const doCreatePromise = SK.broadcasts.create({title: watchBroadcastTitle});
      return Promise.all([createdEventPromise, doCreatePromise])
        .then(function([[newDocs, createdIds], newDoc]) {
          watchBroadcastId = newDoc.id;
          expect(createdIds.indexOf(watchBroadcastId) > -1).to.be.ok;
        });
    });

    it("should recieve updated events", function() {
      // Same deal, now modify it
      watchBroadcastTitle = "Updated in Watch";
      const updateEventPromise = eventPromise("updated");
      const doUpdatePromise = SK.broadcasts.update(watchBroadcastId, {title: watchBroadcastTitle});
      return Promise.all([updateEventPromise, doUpdatePromise])
        .then(function([[newDocs, updatedIds], changedDoc]) {
          expect(updatedIds).to.eql([watchBroadcastId]);
          expect(newDocs.filter(doc => doc.id === watchBroadcastId)).to.eql([changedDoc]);
        });
    });

    it("should recieve deleted events", function() {
      // Same deal, now delete it.
      const deleteEventPromise = eventPromise("deleted");
      const doDeletePromise = SK.broadcasts.delete(watchBroadcastId);
      return Promise.all([deleteEventPromise, doDeletePromise])
        .then(function([[newDocs, removedIds]]) {
          expect(removedIds).to.eql([watchBroadcastId]);
        });
    });

  });


  it("should delete", function() {
    return SK.broadcasts.delete(testBroadcastId)
    .then(function() {
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
