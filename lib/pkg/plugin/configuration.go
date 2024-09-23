package plugin

type Signature struct {
	ID      string `yaml:"id"`      // globally unique id, e.g. ecom-cli.linters.migration-comments
	Version string `yaml:"version"` // semantic version as described at https://pkg.go.dev/golang.org/x/mod/semver
}

type Signatures []Signature

type Config struct {
	Signature
	Requires        Signatures      `yaml:"requires"`
	ExtensionPoints ExtensionPoints `yaml:"extensionPoints"`
}

type ExtensionPoint struct {
	ID     string               `yaml:"id"`     // plugin-level unique id, e.g. migration-comments.processors
	Params ExtensionPointParams `yaml:"params"` // params by name
}

type ExtensionPoints []ExtensionPoint

type ExtensionPointParams map[string]ExtensionPointsParam

type ExtensionPointsParam struct {
	Type  ExtensionPointsParamType `yaml:"type"`  // type
	Array bool                     `yaml:"array"` // true if it is an array
}

type ExtensionPointsParamType string

const (
	ExtensionPointsParamTypeString  ExtensionPointsParamType = "string"
	ExtensionPointsParamTypeInteger ExtensionPointsParamType = "integer"
	ExtensionPointsParamTypeFloat   ExtensionPointsParamType = "float"
	ExtensionPointsParamTypeBoolean ExtensionPointsParamType = "bool"
)

type Extension struct {
	ExtensionPointID  string   `yaml:"id"` // plugin-level unique id, e.g. migration-comments.processors
	BeforePluginIDs   []string `yaml:"beforePluginIDs"`
	AfterPluginIDs    []string `yaml:"afterPluginIDs"`
	CLIImplementation string   `yaml:"cli"`
}
