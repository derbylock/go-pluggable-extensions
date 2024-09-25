# Go pluggable extensions

This is the library which allows creating modular go applications using the WebSocket asa transport.
It spawns plugins as a child processes and allows them declare ExtensionPoints and implement them via Extensions.

## Quick start
Simple example could be found in the [examplecli](./examplecli) folder. It contains app and a plugin.

## TODO:
- [ ] Host extensions
- [ ] Execute extension from plugin API
- [ ] Logging service
- [ ] Progress service

- [ ] Auto update and download plugins
- [ ] Parallel execution `ExecuteExtensionsParallel`

- [ ] Rename `ExecuteExtension` to `ExecuteExtensions`

## Extensions Ordering
When you execute extensions via the `ExecuteExtensions` function, it executes all registered extensions in ordered manner.
The order could be specified by plugins via the `AfterExtensionIDs` and `BeforeExtensionIDs` field of the plugins.

Look at the [plugina](./examplecli/plugina/main.go) for example.

If some plugins have circular dependencies, then `pluginsManager.LoadPlugins` will return error.
Example of the error message:
```
 circular transitive dependency found during plugins extensions priority resolution for extensionID "plugina.hello.welcome". Circular dependency on the extensionID="plugina.hello.currentDate"
```