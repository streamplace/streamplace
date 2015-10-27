
Bellamie.broadcasts = new Mongo.Collection('broadcasts');

Meteor.publish('broadcasts', function() {
  return Bellamie.broadcasts.find();
});
