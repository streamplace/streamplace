
Bellamie.motes = new Mongo.Collection('motes');

Meteor.publish('motes', function(broadcastId) {
  return Bellamie.motes.find({broadcastId});
});
