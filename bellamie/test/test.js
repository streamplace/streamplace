
import SK from "skitchen-client";

SK.broadcasts.find().then(function(broadcasts) {
  // console.log(broadcasts.data);
})
.catch(function(err) {
  // console.error('err', err);
});
