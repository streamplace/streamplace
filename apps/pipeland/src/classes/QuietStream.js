/**
 * Portions of this file were adapted from the excellent libquiet by Brian Armstrong
 * https://github.com/quiet/quiet-js
 *
 * Copyright (c) 2016, Brian Armstrong
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without modification, are permitted
 * provided that the following conditions are met:
 *
 * 1. Redistributions of source code must retain the above copyright notice, this list of
 * conditions and the following disclaimer.
 *
 * 2. Redistributions in binary form must reproduce the above copyright notice, this list of
 * conditions and the following disclaimer in the documentation and/or other materials provided
 * with the distribution.
 *
 * 3. Neither the name of the copyright holder nor the names of its contributors may be used to
 * endorse or promote products derived from this software without specific prior written
 * permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR
 * IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY
 * AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR
 * CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
 * CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
 * SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR
 * OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
 * POSSIBILITY OF SUCH DAMAGE.
 */

import path from "path";
import {Transform} from "stream";

// lol hack hack hack
import Module from "quiet-js";

Module.Runtime.loadDynamicLibrary(path.resolve(__dirname, "../../node_modules/libfec/libfec.js"));

export default class QuietStream extends Transform {
  constructor({profile, sampleRate, sampleBufferSize = 1024, frameBufferSize} = {}) {
    super({highWaterMark: sampleBufferSize * 4});

    if (!profile) {
      throw new Error("Missing profile!");
    }

    // Default parameters
    this.sampleRate = sampleRate || 44100;
    this.sampleBufferSize = sampleBufferSize || 1024;
    this.frameBufferSize = frameBufferSize || Math.pow(2, 14);
    this.arrayBufferSize = sampleBufferSize * 4;

    // Initalize Profile
    const c_profiles = Module.intArrayFromString(JSON.stringify({"profile": profile}));
    const c_profile = Module.intArrayFromString("profile");
    const opt = Module.ccall("quiet_decoder_profile_str", "pointer", ["array", "array"], [
      c_profiles, c_profile
    ]);

    // Pointers to stuff.
    this.decoder = Module.ccall("quiet_decoder_create", "pointer", ["pointer", "number"], [opt, this.sampleRate]);
    this.samples = Module.ccall("malloc", "pointer", ["number"], [4 * this.sampleBufferSize]);
    this.frame = Module.ccall("malloc", "pointer", ["number"], [this.frameBufferSize]);

    this.ab = new ArrayBuffer(this.arrayBufferSize);
    this.abView = new Uint8Array(this.ab);
    this.abIdx = 0;
    this.currentChunk = null;
    this.currentCallback = null;

    // Fail count.
    this.lastChecksumFailCount = 0;
  }

  _transform(chunk, encoding, callback) {
    if (this.currentChunk) {
      throw new Error("AHHH chunk when we had a currentChunk, I dunno what to do");
    }
    let i;
    for (i = 0; i < chunk.length; i+=1) {
      if (this.abIdx >= this.arrayBufferSize) {
        // Our buffer is full!
        this.onFullBuffer();
      }
      this.abView[this.abIdx] = chunk[i];
      this.abIdx += 1;
    }
    callback();
  }

  onReceiveFail(num_fails) {
    this.emit("error", new Error(`OnReceiveFail: num_fails=${num_fails}`));
  }

  onFullBuffer() {
    const sample_view = Module.HEAPF32.subarray(this.samples/4, this.samples/4 + this.sampleBufferSize);
    sample_view.set(new Float32Array(this.ab));
    this.abIdx =  0;
    this.consume();
  }

  consume() {
    Module.ccall("quiet_decoder_consume", "number", ["pointer", "pointer", "number"], [this.decoder, this.samples, this.sampleBufferSize]);
    let currentChecksumFailCount = Module.ccall("quiet_decoder_checksum_fails", "number", ["pointer"], [this.decoder]);
    if (currentChecksumFailCount > this.lastChecksumFailCount) {
      this.onReceiveFail(currentChecksumFailCount);
    }
    this.lastChecksumFailCount = currentChecksumFailCount;
    this.readBuf();
  }

  readBuf() {
    const read = Module.ccall("quiet_decoder_recv", "number", ["pointer", "pointer", "number"], [this.decoder, this.frame, this.frameBufferSize]);
    if (read === -1) {
      return;
    }
    const frameArray = Module.HEAP8.slice(this.frame, this.frame + read);
    this.push(Buffer.from(frameArray.buffer));
    this.readBuf();
  }
}
