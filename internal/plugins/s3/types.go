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
	AWSRegion     string `json:"aws_region" yaml:"aws_region"`
	AWSAccessKey  string `json:"aws_access_key" yaml:"aws_access_key"`
	AWSSecretKey  string `json:"aws_secret_key" yaml:"aws_secret_key"`
	FilePath   string `json:"file_path" yaml:"file_path,omitempty"` // Local file path for upload/download
	S3FileName string `json:"s3_file_name" yaml:"s3_file_name,omitempty"` // Name of the file in S3
	S3FolderName string `json:"s3_folder_name" yaml:"s3_folder_name,omitempty"` // Folder name in S3 bucket
}

type SaveConfig struct {
	FilePath string `json:"file_path" yaml:"file_path,omitempty"`
	As       string `json:"as" yaml:"as"`                         // Variable name to save as
	Required *bool  `json:"required" yaml:"required,omitempty"`   // Whether the value is required (defaults to true)
}

// S3Response represents the response from S3 operations
type S3Response struct {
	Result  map[string]interface{} `json:"result"`
}
	
type ActivityResponse struct {
	Response *S3Response 	 `json:"response"`
	Saved map[string]string  `json:"saved,omitempty"`
}
