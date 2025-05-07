require("@testing-library/jest-dom");

// Polyfill for TextEncoder/TextDecoder in Jest (Node)
const { TextEncoder, TextDecoder } = require("util");
if (typeof global.TextEncoder === "undefined") {
  global.TextEncoder = TextEncoder;
}
if (typeof global.TextDecoder === "undefined") {
  global.TextDecoder = TextDecoder;
}