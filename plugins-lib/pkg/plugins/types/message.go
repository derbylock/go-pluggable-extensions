package pluginstypes

import (
	"encoding/json"
)

type CommandType string

const (
	CommandTypeRegisterPlugin   = "registerPlugin"
	CommandTypeExecuteExtension = "executeExtension"
)

type Message struct {
	Type          CommandType     `json:"command"`                 // type of message
	MsgID         string          `json:"msgID"`                   // unique ID of the message
	CorrelationID string          `json:"correlationID,omitempty"` // empty for requests. equal to request's MsgID fo responses
	Data          json.RawMessage `json:"data,omitempty"`          // data as a generic JSON object
	Error         *PluginError    `json:"error,omitempty"`         // error, could be set in responses if request processing failed because of any reason
	IsFinal       bool            `json:"isFinal,omitempty"`       // is set to true for responses when current response is the last response (when there are multiple responses to a single request)
}

type PluginError struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e PluginError) Error() string {
	return e.Message
}

type RegisterPluginData struct {
	PluginID   string            `json:"pluginID"`
	Secret     string            `json:"secret"`
	Extensions []ExtensionConfig `json:"extensions"`
}

type ExtensionConfig struct {
	ID                 string
	ExtensionPointID   string
	BeforeExtensionIDs []string
	AfterExtensionIDs  []string
}

type RegisterPluginMessage struct {
	Type          CommandType        `json:"command"`
	MsgID         string             `json:"msgID"`
	CorrelationID string             `json:"correlationID,omitempty"`
	Data          RegisterPluginData `json:"data"`
	IsFinal       bool               `json:"isFinal"`
}

type ExecuteExtensionData struct {
	ExtensionPointID string          `json:"extensionPointID"`
	ExtensionID      string          `json:"extensionID"`
	Data             json.RawMessage `json:"data"`
}
