// Spits some data about a hex PES packet
const b = new Buffer(process.argv[2], "hex");

console.log(`Length: ${b.length}`);
function padTo(padLength, number) {
  number = `${number}`;
  while (number.length < padLength) {
    number = `0${number}`;
  }
  return number;
}
function padTo8(number) {
  return padTo(8, number);
}
function dec2bin(dec){
  return (dec >>> 0).toString(2);
}


const startCode = [b.readUInt8(0), b.readUInt8(1), b.readUInt8(2)].map(dec2bin).map(padTo8);
console.log(`Start code: ${startCode}`);

const streamId = padTo8(dec2bin(b.readUInt8(3)));
console.log(`Stream ID: ${streamId}`);

const pesPacketLength = [b.readUInt8(4), b.readUInt8(5)].map(dec2bin).map(padTo8);
console.log(`PES Packet Length: ${pesPacketLength}`);

console.log("-- Begin optional header --");

var head = padTo8(dec2bin(b.readUInt8(6))).split("");
console.log(`Marker bits: ${head[0]}${head[1]}`)
console.log(`Scrambling bits: ${head[2]}${head[3]}`)
console.log(`Priority: ${head[4]}`)
console.log(`Data Alignment Indicator: ${head[5]}`)
console.log(`Copyright: ${head[6]}`)
console.log(`Original or Copy: ${head[7]}`)

head = padTo8(dec2bin(b.readUInt8(7))).split("");
var pts_dts = head[0] + head[1];
var ptsStatus;
if (pts_dts === "11") {
  ptsStatus = "11 (Both present)";
}
else if (pts_dts === "01") {
  ptsStatus = "01 (invalid?!)";
}
else if (pts_dts === "10") {
  ptsStatus = "10 (Only PTS Present)";
}
else if (pts_dts === "00") {
  ptsStatus = "00 (No PTS Present)"
}
console.log(`PTS/DTS: ${ptsStatus}`);
console.log(`ESCR: ${head[2]}`)
console.log(`ES Rate: ${head[3]}`)
console.log(`DSM Trick Mode: ${head[4]}`)
console.log(`Additional Copy Info: ${head[5]}`)
console.log(`CRC: ${head[6]}`)
console.log(`Extension: ${head[7]}`)

const headerLength = b.readUInt8(8);
console.log(`Remaining header length: ${headerLength}`);
var idx = 9;
console.log("--- Remaining Header ---");
for (var i = 0; i < headerLength; i++) {
  console.log(padTo8(dec2bin(b.readUInt8(idx))));
  idx += 1;
}
