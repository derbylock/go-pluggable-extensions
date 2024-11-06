# Go pluggable extensions

This is the library which allows creating modular go applications using the WebSocket as a transport.
It spawns plugins as a child processes and allows them declare ExtensionPoints and implement them via Extensions.

For details about internals, see [Protocol description](./protocol.md)

## Overview
The meaning of Extension Points and Extensions is similar to the meaning of such things in e.g. Eclipse.

That's how it is described:

> A basic rule for building modular software systems is to avoid tight coupling between components. If components are tightly integrated, it becomes difficult to assemble the pieces into different configurations or to replace a component with a different implementation without causing a ripple of changes across the system.
> 
> Loose coupling could be achieved partially through the mechanism of extensions and extension points. The simplest metaphor for describing extensions and extension points is electrical outlets. The outlet, or socket, is the extension point; the plug, or light bulb that connects to it, the extension. As with electric outlets, extension points come in a wide variety of shapes and sizes, and only the extensions that are designed for that particular extension point will fit.
> 
> When a plug-in wants to allow other plug-ins to extend or customize portions of its functionality, it will declare an extension point. ... Plug-ins that want to connect to that extension point must implement its contract in their extension. The key attribute is that the plug-in being extended knows nothing about the plug-in that is connecting to it beyond the scope of that extension point contract. This allows plug-ins built by different individuals or companies to interact seamlessly, even without their knowing much about one another.

It means that your application could be a combination of plugins which extends each others' Extension points.

## Quick start
Simple example could be found in the [examplecli](./examplecli) folder. It contains app and a plugin.

## Extensions Ordering
When you execute extensions via the `ExecuteExtensions` function, it executes all registered extensions in ordered manner.
The order could be specified by plugins via the `AfterExtensionIDs` and `BeforeExtensionIDs` field of the plugins.

Look at the [plugina](./examplecli/plugina/main.go) for example.

If some plugins have circular dependencies, then `pluginsManager.LoadPlugins` will return error.
Example of the error message:
```
 circular transitive dependency found during plugins extensions priority resolution for extensionID "plugina.hello.welcome". Circular dependency on the extensionID="plugina.hello.currentDate"
```

## FAQ
- **Could plugins be implemented using another languages (not go)?**
    
    Yes and no. Currently, this repository contains only library for writing plugins using go.
    You could write your own library using any language and use it (or add PR with it into this repository). For details, see [Protocol description](./protocol.md).
    
    I want to add plugin libraries at least for python, java and javascript. But there are no such libraries at this moment.


- **Why using WebSocket, why not gRPC, TCP, etc.**
    
    Initially I wanted to use gRPC as a transport but the implementation would be more complex.
    The main reasons why WebSocket are: 
    - it is simple
    - message is always atomic. You should not construct it from parts as in TCP.
    - order of messages is preserved and guaranteed for both server and client. For gRPC it is not so easy to control to implement similar behaviour.
    - it has ping/pong functionality which could be used in future for more strict ACK checks


- **Why not [GO plugins](https://pkg.go.dev/plugin)?**
    
    It has many [significant drawbacks](https://pkg.go.dev/plugin#hdr-Warnings) which are frequently important if you want to create modular application without tight coupling.


- **Why not [Hashicorp's go-plugin library](https://github.com/hashicorp/go-plugin)?**
    
    The main reason is that it is mostly about another view on plugins. It is very good library with its pluses, but:
    - it doesn't has simple way for extensions and extension points declaration, so it should be implemented anyway
    - plugins has not simple way to execute host's functions. It could be done, but syntax is not simple enough.

## TODO:

- [ ] Remote plugins: auto update and download remote plugins
- [ ] Parallel execution `ExecuteExtensionsParallel`
