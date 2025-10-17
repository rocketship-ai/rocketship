package log

type LogPlugin struct {
	Name   string    `json:"name" yaml:"name"`
	Plugin string    `json:"plugin" yaml:"plugin"`
	Config LogConfig `json:"config" yaml:"config"`
}

type LogConfig struct {
	Message string `json:"message" yaml:"message"`
}

type LogActivityResponse struct {
	LogMessage string `json:"log_message,omitempty"`
	LogColor   string `json:"log_color,omitempty"`
	LogBold    bool   `json:"log_bold,omitempty"`
}
