import readline from "node:readline";

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  terminal: false,
});

let last;

rl.on("line", (line) => {
  if (line.startsWith("goroutine")) {
    if (last) {
      console.log(JSON.stringify(last));
    }
    last = {
      goroutine: line,
      stack: [`\n${line}`],
    };
  } else {
    last.stack.push(line);
  }
});

rl.on("close", () => {
  if (last) {
    console.log(JSON.stringify(last));
  }
});
