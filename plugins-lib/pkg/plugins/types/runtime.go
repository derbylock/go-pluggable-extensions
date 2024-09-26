package pluginstypes

import "context"

type ExecuteExtensionResult[OUT any] struct {
	Out OUT
	Err error
}

type ExtensionRuntimeInfo struct {
	cfg  ExtensionConfig
	impl ExtensionImplementation[any, any]
}

func NewExtensionRuntimeInfo(
	cfg ExtensionConfig,
	impl ExtensionImplementation[any, any],
) *ExtensionRuntimeInfo {
	return &ExtensionRuntimeInfo{cfg: cfg, impl: impl}
}

func (e *ExtensionRuntimeInfo) Cfg() ExtensionConfig {
	return e.cfg
}

func (e *ExtensionRuntimeInfo) Impl() ExtensionImplementation[any, any] {
	return e.impl
}

type ExtensionImplementation[IN any, OUT any] struct {
	Process     func(ctx context.Context, in IN) (OUT, error)
	Unmarshaler func(bytes []byte) (IN, error)
	Marshaller  func(out OUT) ([]byte, error)
}
