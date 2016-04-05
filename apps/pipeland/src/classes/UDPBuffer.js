/**
 * Okay, here's my first attempt at a UDP buffer. Two arrays with cooresponding indices. One
 * stores the precise time of arrival, one stores a reference to the Buffer. For now I'm just
 * going to have them increase forever and delete old entries when they go away. Eventually I'm
 * going to have to make this much much more efficient, I'm worried about performance here over
 * hours and hours... but for now this'll do just fine I hope.
 */

import EE from "events";
import fs from "fs";
import stream from "stream";
import jDataView from "jdataview";
import mpegtsStream from "../mpegts-stream";
import {SERVER_START_TIME} from "../constants";
var jBinary = require("jbinary");
var MPEGTS = require('mpegts_to_mp4/mpegts_to_mp4/mpegts');
var PES = require('mpegts_to_mp4/mpegts_to_mp4/pes');
// var mpegts_to_mp4 = require('mpegts_to_mp4/mpegts_to_mp4/index');
// import convert from "mpegts_to_mp4";

/**
 * Takes an object with "delay" in milliseconds.
 */
export default class UDPBuffer extends EE {
  constructor({delay}) {
    super();
    this.setDelay(delay);
    this.inputIdx = 0;
    this.outputIdx = 0;
    this.times = [];
    this.buffers = [];

    // Register myself statically, so one setInterval can hit all the buffers.
    UDPBuffer.buffers.push(this);
  }

  /**
   * Takes delay in millis.
   */
  setDelay(delay) {
    this.delayS = Math.floor(delay / 1000); // ms to s.
    this.delayNS = (delay - (this.delayS * 1000)) * 1000000; // ms to ns.
  }

  push(buffer) {
    this.times[this.inputIdx] = process.hrtime();
    this.buffers[this.inputIdx] = buffer;
    try {
      const mpegts = new jBinary(buffer, MPEGTS);
      const packets = mpegts.read("File");
      for (let i = 0, length = packets.length; i < length; i++) {
        let packet = packets[i];
        if (packet.payload && packet.payload._rawStream) {
          let pesStream = new jBinary(packet.payload._rawStream, PES);
          let byte = pesStream.read("uint8", 0);
          if (byte !== 0) {
            continue;
          }
          byte = pesStream.read("uint8", 1);
          if (byte !== 0) {
            continue;
          }
          let pesPacket = pesStream.read("PESPacket", 0);
          const newTime = (((new Date()).getTime() * 1000 + (this.delayS * 1000 * 1000)) - SERVER_START_TIME) / (0.001000 * 10000);
          if (pesPacket.dts) {
            let dtsLocation = pesPacket.pts ? 14 : 9;
            let timeStamp = pesStream.read(["PESTimeStamp", 1], dtsLocation);
            // timeStamp += (this.delayS * 180000);
            pesStream.write(["PESTimeStamp", 1], newTime, dtsLocation);
          }
          if (pesPacket.pts) {
            // let log = [];
            // log.push(`PTS: ${pesPacket.pts}, DTS: ${pesPacket.dts}`);
            // log.push(`Data Length: ${pesPacket.dataLength}`);
            // log.push("Before:");
            // let before = pesStream.view.buffer.toString("hex");
            // log.push(before);

            let startByte = pesPacket.dts ? 3 : 2;
            let timeStamp = pesStream.read(["PESTimeStamp", startByte], 9);
            // console.log("Old Timestamp:" + timeStamp);
            // console.log("New Timestamp:" + newTime);
            // console.log(timeStamp);
            // timeStamp += (this.delayS * 180000);
            pesStream.write(["PESTimeStamp", startByte], newTime, 9);
            // log.push(typeof pesPacket.pts);
            // let first = pesPacket.data.toString("hex");
            // pesPacket.pts = ;
            // pesStream.write("PESPacket", pesPacket, 0);
            // pesPacket = pesStream.read("PESPacket", 0);
            // let second = pesPacket.data.toString("hex");
            // log.push("After:");
            // let after = pesStream.view.buffer.toString("hex");
            // log.push(after);
            // log.push("next");
            // if (before !== after) {
            //   console.log(log.join("\n"));
            //   process.exit(0);
            // }
          }
        }
      }
    }
    catch (e) {
      console.log(e.stack);
    }
    // const bufferStream = new stream.PassThrough();
    // bufferStream.end(buffer);
    // try {
    //   jBinary.load(bufferStream, MPEGTS, function (err, mpegts) {
    //     if (err) {
    //       console.log(err);
    //       process.exit(1);
    //     }
    //     try {
    //       const packets = mpegts.read("File");
    //       console.log(mpegts.view.byteLength);
    //       var stream = new jDataView(mpegts.view.byteLength);
    //       for (var i = 0, length = packets.length; i < length; i++) {
    //         var packet = packets[i], adaptation = packet.adaptationField, payload = packet.payload;
    //         if (payload && payload._rawStream) {
    //           stream.writeBytes(payload._rawStream);
    //         }
    //       }
    //       try {
    //         var pesStream = new jBinary(stream, PES);
    //         console.log(pesStream.view.byteLength);
    //         while (pesStream.tell() < pesStream.view.byteLength) {
    //           var packet = pesStream.read("PESPacket");
    //           if (packet.pts) {
    //             console.log(packet.pts);
    //           }
    //         }
    //       }
    //       catch (e) {
    //         console.log("Error parsing PTS stream");
    //       }
    //     }
    //     catch (e) {
    //       console.log("Error parsing packets");
    //       console.log(e.stack);
    //       process.exit(1);
    //     }
    //   });
    // }
    // catch (e) {
    //   console.log("Error loading mpegts stream");
    //   console.log(e.stack);
    //   process.exit(1);
    // }
    this.inputIdx += 1;
  }

  /**
   * Check if the provided diff exceeds our stored delay.
   */
  timeExceedsDelay(time) {
    return true;
    const [s, ns] = process.hrtime(time);
    if (s > this.delayS) {
      return true;
    }
    else if (s === this.delayS) {
      return ns >= this.delayNS;
    }
    return false;
  }

  handleTick() {
    let packetTime = this.times[this.outputIdx];
    while (packetTime && this.timeExceedsDelay(packetTime)) {
      this.sendPacket();
      // SendPacket increments our outputIdx, so...
      packetTime = this.times[this.outputIdx];
    }
  }

  sendPacket() {
    const outputBuffer = this.buffers[this.outputIdx];
    if (!outputBuffer) {
      throw new Error("Tried to send a packet we didn't have!");
    }
    delete this.buffers[this.outputIdx];
    delete this.times[this.outputIdx];
    this.outputIdx += 1;
    this.emit("message", outputBuffer);
  }
}

UDPBuffer.TICK_RATE = 3;
UDPBuffer.buffers = [];
setInterval(function() {
  UDPBuffer.buffers.forEach(function(buffer) {
    buffer.handleTick();
  });
}, UDPBuffer.TICK_RATE);

setInterval(function() {
  UDPBuffer.buffers.forEach(function(buffer) {
    const size = buffer.inputIdx - buffer.outputIdx;
    buffer.emit("info", {size});
  });
}, UDPBuffer.TICK_RATE * 100);
