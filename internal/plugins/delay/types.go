package delay

type DelayPlugin struct {
	Name   string      `json:"name" yaml:"name"`
	Plugin string      `json:"plugin" yaml:"plugin"`
	Config DelayConfig `json:"config" yaml:"config"`
}

type DelayConfig struct {
	Duration string `json:"duration" yaml:"duration"`
}
