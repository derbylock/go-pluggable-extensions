package pluginstypes

import "context"

// ExecuteExtensionResult is a struct that contains the result of executing an extension.
//
// OUT is the type of the output of the extension.
type ExecuteExtensionResult[OUT any] struct {
	Out OUT
	Err error
}

// ExtensionRuntimeInfo is a struct that contains information about an extension.
//
// cfg is the configuration of the extension.
//
// impl is the implementation of the extension.
type ExtensionRuntimeInfo struct {
	cfg  ExtensionConfig
	impl ExtensionImplementation[any, any]
}

// NewExtensionRuntimeInfo creates a new ExtensionRuntimeInfo struct.
//
// cfg is the configuration of the extension.
//
// impl is the implementation of the extension.
func NewExtensionRuntimeInfo(
	cfg ExtensionConfig,
	impl ExtensionImplementation[any, any],
) *ExtensionRuntimeInfo {
	return &ExtensionRuntimeInfo{cfg: cfg, impl: impl}
}

// Cfg returns the configuration of the extension.
func (e *ExtensionRuntimeInfo) Cfg() ExtensionConfig {
	return e.cfg
}

// Impl returns the implementation of the extension.
func (e *ExtensionRuntimeInfo) Impl() ExtensionImplementation[any, any] {
	return e.impl
}

// ExtensionImplementation is a struct that contains the implementation of an extension.
//
// Process is a function that takes a context and an input and returns an output and an error.
//
// Unmarshaler is a function that takes a byte slice and returns an input and an error.
//
// Marshaller is a function that takes an output and returns a byte slice and an error.
type ExtensionImplementation[IN any, OUT any] struct {
	Process     func(ctx context.Context, in IN) (OUT, error)
	Unmarshaler func(bytes []byte) (IN, error)
	Marshaller  func(out OUT) ([]byte, error)
}
