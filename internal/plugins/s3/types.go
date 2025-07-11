package s3

type S3Plugin struct {
	Name	string `json:"name" yaml:"name"`
	Plugin  string `json:"plugin" yaml:"plugin"`
	Config  S3Config `json:"config" yaml:"config"`
	// Assertions []string `json:"assertions,omitempty" yaml:"assertions,omitempty"`
	// Save    []SaveConfig `json:"save" yaml:"save, omitempty"`
}

// S3Config represents the configuration for S3 operations
type S3Config struct {
	Operation  string `json:"operation" yaml:"operation"`
	Bucket     string `json:"bucket" yaml:"bucket"`
	Region     string `json:"region" yaml:"region"`
	AccessKey  string `json:"access_key" yaml:"access_key"`
	SecretKey  string `json:"secret_key" yaml:"secret_key"`
	FilePath   string `json:"file_path" yaml:"file_path,omitempty"` // Local file path for upload/download
	FileName   string `json:"file_name" yaml:"file_name,omitempty"` // Name of the file in S3
	FolderName string `json:"folder_name" yaml:"folder_name,omitempty"` // Folder name in S3 bucket
}

type SaveConfig struct {
	FilePath string `json:"file_path" yaml:"file_path,omitempty"`
	As       string `json:"as" yaml:"as"`                         // Variable name to save as
	Required *bool  `json:"required" yaml:"required,omitempty"`   // Whether the value is required (defaults to true)
}

// S3Response represents the response from S3 operations
type S3Response struct {
	URL     string `json:"url,omitempty" yaml:"url,omitempty"` // URL of the uploaded file if applicable
}
