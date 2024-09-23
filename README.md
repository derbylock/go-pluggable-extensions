# Go pluggable extensions

This is the library which allows creating modular go applications using the WebSocket asa transport.
It spawns plugins as a child processes and allows them declare ExtensionPoints and implement them via Extensions.

Simple example could be found in the [examplecli](./examplecli) folder. It contains app and a plugin.

## TODO:
- [ ] Host extensions
- [ ] Execute extension from plugin API
- [ ] Logging service
- [ ] Progress service

- [ ] Extensions Ordering
