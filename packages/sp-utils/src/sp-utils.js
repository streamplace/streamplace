// I rather like Meteor's random ids, so we're using them here. MIT-licensed as follows.

// Copyright (C) 2011--2016 Meteor Development Group

// Permission is hereby granted, free of charge, to any person obtaining a copy of this software
// and associated documentation files (the "Software"), to deal in the Software without
// restriction, including without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all copies or
// substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING
// BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
// DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

const UNMISTAKABLE_CHARS =
  "23456789ABCDEFGHJKLMNPQRSTWXYZabcdefghijkmnopqrstuvwxyz";

const choice = function(arrayOrString) {
  const index = Math.floor(Math.random() * arrayOrString.length);
  if (typeof arrayOrString === "string") {
    return arrayOrString.substr(index, 1);
  } else {
    return arrayOrString[index];
  }
};

const randomString = function(charsCount, alphabet) {
  const digits = [];
  for (let i = 0; i < charsCount; i++) {
    digits[i] = choice(alphabet);
  }
  return digits.join("");
};

export function randomId(charsCount) {
  if (charsCount === undefined) {
    charsCount = 19;
  }

  return randomString(charsCount, UNMISTAKABLE_CHARS);
}

/**
 * Converts (0, 0, 960, 520, 1920, 1080) to (-480, 270).
 *
 * I know that this depends on the projection matrix, but that's really confusing so here we are
 */
export function relativeCoords(
  x,
  y,
  myWidth,
  myHeight,
  canvasWidth,
  canvasHeight
) {
  x += myWidth / 2;
  x -= canvasWidth / 2;
  y += myHeight / 2;
  y -= canvasHeight / 2;
  y = y * -1;
  return [x, y];
}
