function reverse(s) {
  return s.split("").reverse().join("");
}

// Verifies that types from the template are correctly propagated through JS and
// back to Go.
function hello(c) {
  return goHello(c);
}
