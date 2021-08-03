package constants

const (
	AnonymousTokenResponse            = "{\"Username\":\"\",\"Secret\":\"\"}\n"
	BinariesSubdir                    = "bin"
	DockerCredentialPrefix            = "docker-credential"
	DockerConfigFileBasename          = "config.json"
	DockerConfigFileContents          = "{\"credsStore\":\"magic\"}\n"
	EmbeddedParentDir                 = "embedded"
	EnvVarDockerConfig                = "DOCKER_CONFIG"
	EnvVarDockerCredentialMagicConfig = "DOCKER_CREDENTIAL_MAGIC_CONFIG"
	EnvVarPath                        = "PATH"
	ExtensionYAML                     = "yml"
	HelperSubcommandGet               = "get"
	MagicCredentialSuffix             = "magic"
	MagicRootDir                      = "/opt/magic"
	MappingsSubdir                    = "etc"
	XDGConfigSubdir                   = "magic"
)
