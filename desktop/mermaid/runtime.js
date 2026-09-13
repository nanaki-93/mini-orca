globalThis.global = globalThis;
// The entity table uses browser atob; the embedded engine has no browser globals.
globalThis.atob = (input) => {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  let bits = 0, accumulator = 0, result = '';
  for (const character of input.replace(/=+$/, '')) {
    const value = alphabet.indexOf(character);
    if (value < 0) throw new Error('Invalid base64 in bundled entity table');
    accumulator = (accumulator << 6) | value;
    bits += 6;
    if (bits >= 8) { bits -= 8; result += String.fromCharCode((accumulator >> bits) & 255); }
  }
  return result;
};
// ELK's synchronous API also queues a zero-delay setup callback.
globalThis.setTimeout = (callback, delay = 0) => {
  if (delay !== 0) throw new Error('Delayed work is unsupported in the diagram renderer');
  callback();
  return 0;
};
