package plugins

import (
	"encoding/json"
	"fmt"
)

type CommandType string

const (
	CommandTypeRegisterPlugin   = "registerPlugin"
	CommandTypeExecuteExtension = "executeExtension"
)

type Message struct {
	Type          CommandType     `json:"command"`
	MsgID         string          `json:"msgID"`
	CorrelationID string          `json:"correlationID,omitempty"`
	Data          json.RawMessage `json:"data,omitempty"`
	Error         *PluginError    `json:"error,omitempty"`
	IsFinal       bool            `json:"isFinal,omitempty"`
}

type PluginError struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e PluginError) Error() string {
	return fmt.Sprintf("plugin error %s: %s", e.Type, e.Message)
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
