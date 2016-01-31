
import StreamKitchenClient from "skitchen-client";

const SK = new StreamKitchenClient({server: "http://localhost:80"});

SK.broadcasts.find().then(function(broadcasts) {
  console.log(broadcasts.data);
})
.catch(function(err) {
  console.error("err", err);
});
