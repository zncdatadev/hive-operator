package v1alpha1

//type LoggingSpec struct {
//	// +kubebuilder:validation:Optional
//	Containers *ContainerLoggingSpec `json:"containers,omitempty"`
//}

type ContainerLoggingSpec struct {
	// +kubebuilder:validation:Optional
	Metastore *LoggingConfigSpec `json:"metastore,omitempty"`
}

type LoggingConfigSpec struct {
	// +kubebuilder:validation:Optional
	Loggers map[string]*LogLevelSpec `json:"loggers,omitempty"`

	// +kubebuilder:validation:Optional
	Console *LogLevelSpec `json:"console,omitempty"`

	// +kubebuilder:validation:Optional
	File *LogLevelSpec `json:"file,omitempty"`
}

// LogLevelSpec
// level mapping example
// |---------------------|-----------------|
// |  superset log level |  zds log level  |
// |---------------------|-----------------|
// |  CRITICAL           |  FATAL          |
// |  ERROR              |  ERROR          |
// |  WARNING            |  WARN           |
// |  INFO               |  INFO           |
// |  DEBUG              |  DEBUG          |
// |  DEBUG              |  TRACE          |
// |---------------------|-----------------|
type LogLevelSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="INFO"
	// +kubebuilder:validation:Enum=FATAL;ERROR;WARN;INFO;DEBUG;TRACE
	Level string `json:"level,omitempty"`
}
