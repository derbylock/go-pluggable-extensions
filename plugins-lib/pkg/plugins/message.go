package plugins

import "encoding/json"

type CommandType string

const (
	CommandTypeRegisterPlugin   = "registerPlugin"
	CommandTypeExecuteExtension = "executeExtension"
)

type Message struct {
	Type          CommandType     `json:"command"`
	MsgID         string          `json:"msgID"`
	CorrelationID string          `json:"correlationID"`
	Data          json.RawMessage `json:"data"`
	IsFinal       bool            `json:"isFinal"`
}

type RegisterPluginData struct {
	PluginID   string            `json:"pluginID"`
	Secret     string            `json:"secret"`
	Extensions []ExtensionConfig `json:"extensions"`
}

type ExtensionConfig struct {
	ID               string
	ExtensionPointID string
	BeforeExtensions string
	AfterExtensions  string
}

type RegisterPluginMessage struct {
	Type          CommandType        `json:"command"`
	MsgID         string             `json:"msgID"`
	CorrelationID string             `json:"correlationID"`
	Data          RegisterPluginData `json:"data"`
	IsFinal       bool               `json:"isFinal"`
}

type ExecuteExtensionData struct {
	ExtensionID string          `json:"extensionID"`
	Data        json.RawMessage `json:"data"`
}
