package pluginstypes

import (
	"encoding/json"
)

// CommandType is a type of message.
type CommandType string

const (
	// CommandTypeRegisterPlugin is a command to register a plugin.
	CommandTypeRegisterPlugin = "registerPlugin"
	// CommandTypeExecuteExtension is a command to execute an extension or return its result.
	CommandTypeExecuteExtension = "executeExtension"
)

// Message is a message that can be sent or received.
type Message struct {
	// Type is the type of message.
	Type CommandType `json:"command"`
	// MsgID is a unique ID of the message.
	MsgID string `json:"msgID"`
	// CorrelationID is a unique ID of the message that is used to correlate requests and responses.
	CorrelationID string `json:"correlationID,omitempty"`
	// Data is the data of the message as a generic JSON object.
	Data json.RawMessage `json:"data,omitempty"`
	// Error is an error that occurred during the processing of the message.
	Error *PluginError `json:"error,omitempty"`
	// IsFinal is a flag that indicates whether the message is the last message in a sequence of messages.
	// It is set to true for responses when current response is the last response
	// (when there are multiple responses to a single request).
	IsFinal bool `json:"isFinal,omitempty"`
}

// PluginError is an error that occurred during the extension's execution.
type PluginError struct {
	// Type is the type of error.
	Type string `json:"type,omitempty"`
	// Message is a message that describes the error.
	Message string `json:"message,omitempty"`
}

// Error returns a string representation of the error.
func (e PluginError) Error() string {
	return e.Message
}

// RegisterPluginData is the data that is sent with a registerPlugin command.
type RegisterPluginData struct {
	// PluginID is the ID of the plugin.
	PluginID string `json:"pluginID"`
	// Secret is a secret that is used to authenticate the plugin.
	Secret string `json:"secret"`
	// Extensions is a list of extensions that the plugin provides.
	Extensions []ExtensionConfig `json:"extensions"`
}

// ExtensionConfig is the configuration of an extension.
type ExtensionConfig struct {
	// ID is the ID of the extension.
	ID string
	// ExtensionPointID is the ID of the extension point that the extension implements.
	ExtensionPointID string
	// BeforeExtensionIDs is a list of IDs of extensions that the extension should be executed before.
	BeforeExtensionIDs []string
	// AfterExtensionIDs is a list of IDs of extensions that the extension should be executed after.
	AfterExtensionIDs []string
}

// RegisterPluginMessage is a message that is sent to register a plugin.
type RegisterPluginMessage struct {
	// Type is the type of message.
	Type CommandType `json:"command"`
	// MsgID is a unique ID of the message.
	MsgID string `json:"msgID"`
	// CorrelationID is a unique ID of the message that is used to correlate requests and responses.
	CorrelationID string `json:"correlationID,omitempty"`
	// Data is the data of the message as a generic JSON object.
	Data RegisterPluginData `json:"data"`
	// IsFinal is a flag that indicates whether the message is the last message in a sequence of messages.
	IsFinal bool `json:"isFinal"`
}

// ExecuteExtensionData is the data that is sent with an executeExtension command.
type ExecuteExtensionData struct {
	// ExtensionPointID is the ID of the extension point that the extension is registered to.
	ExtensionPointID string `json:"extensionPointID"`
	// ExtensionID is the ID of the extension that should be executed.
	ExtensionID string `json:"extensionID"`
	// Data is the data that should be passed to the extension.
	Data json.RawMessage `json:"data"`
}
