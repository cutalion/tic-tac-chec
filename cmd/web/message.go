package main

// TODO(human): Define the inbound message envelope type.
//
// Every client message has:
//   { "ch": "lobby", "event": "join", ...payload }
//
// Design the struct that lets you:
// 1. Unmarshal any message to read ch + event
// 2. Defer parsing of the remaining fields until you know what they are
