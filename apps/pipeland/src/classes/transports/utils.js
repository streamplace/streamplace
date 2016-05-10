
/**
 * Get a random number between min and max. Inclusive of min, exclusive of max.
 * @param  {Number} min
 * @param  {Number} max
 * @return {Number}
 */
export function getRandomArbitrary(min, max) {
  return Math.floor(Math.random() * (max - min) + min);
}

/**
 * Get a random port.
 * @return {[type]} [description]
 */
export function randomPort() {
  return getRandomArbitrary(40000, 50000);
}
