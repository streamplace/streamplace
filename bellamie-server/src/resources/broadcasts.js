
import router from "koa-router";

export default router()

.get("/", function * getBroadcasts(next) {
  this.body = "Here are some broadcasts";
  yield next;
})

.post("/", function * createBroadcast(next) {
  this.body = "Good job, you made a broadcast";
  yield next;
})

.put("/:id", function * updateBroadcast(next) {
  this.body = "You sure are good at editing broadcasts!";
  yield next;
})

.delete("/:id", function * deleteBroadcast(next) {
  this.body = "Bye bye broadcast!";
  yield next;
});
