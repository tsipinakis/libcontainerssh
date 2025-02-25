package config

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sYaml "sigs.k8s.io/yaml"
)

// KubernetesConfig is the base configuration structure for Kubernetes
type KubernetesConfig struct {
	// Connection configures the connection to the Kubernetes cluster.
	Connection KubernetesConnectionConfig `json:"connection,omitempty" yaml:"connection" comment:"Kubernetes configuration options"`
	// Pod contains the spec and specific settings for creating the pod.
	Pod KubernetesPodConfig `json:"pod,omitempty" yaml:"pod" comment:"Container configuration"`
	// Timeout specifies how long to wait for the Pod to come up.
	Timeouts KubernetesTimeoutConfig `json:"timeouts,omitempty" yaml:"timeouts" comment:"Timeout for pod creation"`
}

// Validate checks the configuration options and returns an error if the configuration is invalid.
func (c KubernetesConfig) Validate() error {
	if err := c.Connection.Validate(); err != nil {
		return err
	}
	if err := c.Pod.Validate(); err != nil {
		return err
	}
	if err := c.Timeouts.Validate(); err != nil {
		return err
	}
	return nil
}

// KubernetesConnectionConfig configures the connection to the Kubernetes cluster.
//goland:noinspection GoVetStructTag
type KubernetesConnectionConfig struct {
	// Host is a host string, a host:port pair, or a URL to the Kubernetes apiserver. Defaults to kubernetes.default.svc.
	Host string `json:"host,omitempty" yaml:"host" comment:"a host string, a host:port pair, or a URL to the base of the apiserver." default:"kubernetes.default.svc"`
	// APIPath is a sub-path that points to the API root. Defaults to /api
	APIPath string `json:"path,omitempty" yaml:"path" comment:"APIPath is a sub-path that points to an API root." default:"/api"`

	// Username is the username for basic authentication.
	Username string `json:"username,omitempty" yaml:"username" comment:"Username for basic authentication"`
	// Password is the password for basic authentication.
	Password string `json:"password,omitempty" yaml:"password" comment:"Password for basic authentication"`

	// ServerName sets the server name to be set in the SNI and used by the client for TLS verification.
	ServerName string `json:"serverName,omitempty" yaml:"serverName" comment:"ServerName is passed to the server for SNI and is used in the client to check server certificates against."`

	// CertFile points to a file that contains the client certificate used for authentication.
	CertFile string `json:"certFile,omitempty" yaml:"certFile" comment:"File containing client certificate for TLS client certificate authentication."`
	// KeyFile points to a file that contains the client key used for authentication.
	KeyFile string `json:"keyFile,omitempty" yaml:"keyFile" comment:"File containing client key for TLS client certificate authentication"`
	// CAFile points to a file that contains the CA certificate for authentication.
	CAFile string `json:"cacertFile,omitempty" yaml:"cacertFile" comment:"File containing trusted root certificates for the server"`

	// CertData contains a PEM-encoded certificate for TLS client certificate authentication.
	CertData string `json:"cert,omitempty" yaml:"cert" comment:"PEM-encoded certificate for TLS client certificate authentication"`
	// KeyData contains a PEM-encoded client key for TLS client certificate authentication.
	KeyData string `json:"key,omitempty" yaml:"key" comment:"PEM-encoded client key for TLS client certificate authentication"`
	// CAData contains a PEM-encoded trusted root certificates for the server.
	CAData string `json:"cacert,omitempty" yaml:"cacert" comment:"PEM-encoded trusted root certificates for the server"`

	// BearerToken contains a bearer (service) token for authentication.
	BearerToken string `json:"bearerToken,omitempty" yaml:"bearerToken" comment:"Bearer (service token) authentication"`
	// BearerTokenFile points to a file containing a bearer (service) token for authentication.
	// Set to /var/run/secrets/kubernetes.io/serviceaccount/token to use service token in a Kubernetes kubeConfigCluster.
	BearerTokenFile string `json:"bearerTokenFile,omitempty" yaml:"bearerTokenFile" comment:"Path to a file containing a BearerToken. Set to /var/run/secrets/kubernetes.io/serviceaccount/token to use service token in a Kubernetes kubeConfigCluster."`

	// QPS indicates the maximum QPS to the master from this client. Defaults to 5.
	QPS float32 `json:"qps,omitempty" yaml:"qps" comment:"QPS indicates the maximum QPS to the master from this client." default:"5"`
	// Burst indicates the maximum burst for throttle.
	Burst int `json:"burst,omitempty" yaml:"burst" comment:"Maximum burst for throttle." default:"10"`
}

func (c KubernetesConnectionConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("no host specified")
	}
	if c.APIPath == "" {
		return fmt.Errorf("no API path specified")
	}
	if c.BearerTokenFile != "" {
		if _, err := os.Stat(c.BearerTokenFile); err != nil {
			return fmt.Errorf("bearer token file %s not found (%w)", c.BearerTokenFile, err)
		}
	}
	return nil
}

// KubernetesPodConfig describes the pod to launch.
//goland:noinspection GoVetStructTag
type KubernetesPodConfig struct {
	// Metadata configures the pod metadata.
	Metadata metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty" default:"{\"namespace\":\"default\",\"generateName\":\"containerssh-\"}"`
	// Spec contains the pod specification to launch.
	Spec v1.PodSpec `json:"spec,omitempty" yaml:"spec" comment:"Pod specification to launch" default:"{\"containers\":[{\"name\":\"shell\",\"image\":\"containerssh/containerssh-guest-image\"}]}"`

	// ConsoleContainerNumber specifies the container to attach the running process to. Defaults to 0.
	ConsoleContainerNumber int `json:"consoleContainerNumber,omitempty" yaml:"consoleContainerNumber" comment:"Which container to attach the SSH connection to" default:"0"`

	// IdleCommand contains the command to run as the first process in the container. Other commands are executed using the "exec" method.
	IdleCommand []string `json:"idleCommand,omitempty" yaml:"idleCommand" comment:"Run this command to wait for container exit" default:"[\"/usr/bin/containerssh-agent\", \"wait-signal\", \"--signal\", \"INT\", \"--signal\", \"TERM\"]"`
	// ShellCommand is the command used for launching shells when the container. Required in KubernetesExecutionModeConnection and when the agent is used.
	ShellCommand []string `json:"shellCommand,omitempty" yaml:"shellCommand" comment:"Run this command as a default shell." default:"[\"/bin/bash\"]"`
	// AgentPath contains the path to the ContainerSSH Guest Agent.
	AgentPath string `json:"agentPath,omitempty" yaml:"agentPath" default:"/usr/bin/containerssh-agent"`
	// DisableAgent disables using the ContainerSSH Guest Agent.
	DisableAgent bool `json:"disableAgent,omitempty" yaml:"disableAgent"`
	// Subsystems contains a map of subsystem names and the executable to launch.
	Subsystems map[string]string `json:"subsystems,omitempty" yaml:"subsystems" comment:"Subsystem names and binaries map." default:"{\"sftp\":\"/usr/lib/openssh/sftp-server\"}"`

	// Mode influences how commands are executed.
	//
	// - If KubernetesExecutionModeConnection is chosen (default) a new pod is launched per connection. In this mode
	//   sessions are executed using the "docker exec" functionality and the main container console runs a script that
	//   waits for a termination signal.
	// - If KubernetesExecutionModeSession is chosen a new pod is launched per session, leading to potentially multiple
	//   pods per connection. In this mode the program is launched directly as the main process of the container.
	//   When configuring this mode you should explicitly configure the "cmd" option to an empty list if you want the
	//   default command in the container to launch.
	Mode KubernetesExecutionMode `json:"mode,omitempty" yaml:"mode" default:"connection"`

	// ExposeAuthMetadataAsEnv causes the specified metadata entries received from the authentication process to be
	// exposed as environment variables. They are provided as a map, where the key is the authentication metadata entry
	// name and the value is the environment variable. The default is to expose no authentication metadata.
	ExposeAuthMetadataAsEnv map[string]string `json:"exposeAuthMetadataAsEnv" yaml:"exposeAuthMetadataAsEnv"`

	// ExposeAuthMetadataAsLabels causes the specified metadata entries received from the authentication process to be
	// exposed in the pod labels. They are provided as a map, where the key is the authentication metadata entry name
	// and the value is the label name. The label name must conform to Kubernetes label name requirements or the pod
	// will not start. The default is to expose no labels.
	ExposeAuthMetadataAsLabels map[string]string `json:"exposeAuthMetadataAsLabels" yaml:"exposeAuthMetadataAsLabels"`

	// ExposeAuthMetadataAsAnnotations causes the specified metadata entries received from the authentication process to
	// be exposed in the pod annotations. They are provided as a map, where the key is the authentication metadata entry
	// name and the value is the annotation name. The annotation name must conform to Kubernetes annotation name
	// requirements or the pod will not start. The default is to expose no annotations.
	ExposeAuthMetadataAsAnnotations map[string]string `json:"exposeAuthMetadataAsAnnotations" yaml:"exposeAuthMetadataAsAnnotations"`
}

// Validate validates the pod configuration.
func (c KubernetesPodConfig) Validate() error {
	if c.Metadata.Namespace == "" {
		return fmt.Errorf("no namespace specified in pod config")
	}
	if c.ConsoleContainerNumber >= len(c.Spec.Containers) {
		return fmt.Errorf("the specified container for consoles does not exist in the pod spec")
	}
	if !c.DisableAgent {
		if c.AgentPath == "" {
			return fmt.Errorf("the agent path is required when the agent is not disabled")
		}
	}
	if len(c.Spec.Containers) == 0 {
		return fmt.Errorf("no containers specified in the pod spec")
	}
	for i, container := range c.Spec.Containers {
		if container.Image == "" {
			return fmt.Errorf("container %d in pod spec has no image name", i)
		}
	}
	if err := c.Mode.Validate(); err != nil {
		return err
	}
	if c.Mode == KubernetesExecutionModeConnection {
		if len(c.IdleCommand) == 0 {
			return fmt.Errorf("idle command is required when the execution mode is connection")
		}
		if len(c.ShellCommand) == 0 {
			return fmt.Errorf("shell command is required when the execution mode is connection")
		}
	} else if c.Mode == KubernetesExecutionModeSession {
		if c.Spec.RestartPolicy != "" && c.Spec.RestartPolicy != v1.RestartPolicyNever {
			return fmt.Errorf(
				"invalid restart policy in session mode: %s only \"Never\" is allowed",
				c.Spec.RestartPolicy,
			)
		}
		if !c.DisableAgent && len(c.ShellCommand) == 0 {
			return fmt.Errorf("shell command is required when using the agent")
		}
	}
	return nil
}

// MarshalYAML uses the Kubernetes YAML library to encode the KubernetesPodConfig instead of the default configuration.
func (c KubernetesPodConfig) MarshalYAML() (interface{}, error) {
	data, err := k8sYaml.Marshal(c)
	if err != nil {
		return nil, err
	}
	node := map[string]yaml.Node{}
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}
	return node, nil
}

// UnmarshalYAML uses the Kubernetes YAML library to encode the KubernetesPodConfig instead of the default configuration.
func (c *KubernetesPodConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	node := map[string]yaml.Node{}
	if err := unmarshal(&node); err != nil {
		return err
	}
	data, err := yaml.Marshal(node)
	if err != nil {
		return err
	}
	if err := k8sYaml.UnmarshalStrict(data, c); err != nil {
		return err
	}
	return nil
}

// KubernetesTimeoutConfig configures the various timeouts for the Kubernetes backend.
type KubernetesTimeoutConfig struct {
	// PodStart is the timeout for creating and starting the pod.
	PodStart time.Duration `json:"podStart,omitempty" yaml:"podStart" default:"60s"`
	// PodStop is the timeout for stopping and removing the pod.
	PodStop time.Duration `json:"podStop,omitempty" yaml:"podStop" default:"60s"`
	// CommandStart sets the maximum time starting a command may take.
	CommandStart time.Duration `json:"commandStart,omitempty" yaml:"commandStart" default:"60s"`
	// Signal sets the maximum time sending a signal may take.
	Signal time.Duration `json:"signal,omitempty" yaml:"signal" default:"60s"`
	// Signal sets the maximum time setting the window size may take.
	Window time.Duration `json:"window,omitempty" yaml:"window" default:"60s"`
	// HTTP configures the timeout for HTTP calls
	HTTP time.Duration `json:"http,omitempty" yaml:"http" default:"15s"`
}

// Validate validates the timeout configuration.
func (c KubernetesTimeoutConfig) Validate() error {
	return nil
}

// KubernetesExecutionMode determines when a container is launched.
// KubernetesExecutionModeConnection launches one container per SSH connection (default), while
// KubernetesExecutionModeSession launches one container per SSH session.
type KubernetesExecutionMode string

const (
	// KubernetesExecutionModeConnection launches one container per SSH connection.
	KubernetesExecutionModeConnection KubernetesExecutionMode = "connection"
	// KubernetesExecutionModeSession launches one container per SSH session (multiple containers per connection).
	KubernetesExecutionModeSession KubernetesExecutionMode = "session"
)

// Validate validates the execution config.
func (e KubernetesExecutionMode) Validate() error {
	switch e {
	case KubernetesExecutionModeConnection:
		fallthrough
	case KubernetesExecutionModeSession:
		return nil
	default:
		return fmt.Errorf("invalid execution mode: %s", e)
	}
}

// SetConfigFromKubeConfig reads the kubeconfig of the current system user and sets the variables accordingly.
// This method is meant mainly for testing and not all kubeconfig combinations are supported.
func (c *KubernetesConfig) SetConfigFromKubeConfig() (err error) {
	kubeConfigUser, kubeConfigCluster, err := getKubeConfigDetails()
	if err != nil {
		return err
	}

	c.Connection.Host = strings.Replace(
		kubeConfigCluster.Cluster.Server,
		"https://",
		"",
		1,
	)
	if err = c.Connection.configureCertificates(kubeConfigCluster, kubeConfigUser); err != nil {
		return err
	}

	return nil
}

func (c *KubernetesConnectionConfig) configureCertificates(
	kubeConfigCluster *kubeConfigCluster,
	kubeConfigUser *kubeConfigUser,
) error {
	decodedCa, err := base64.StdEncoding.DecodeString(
		kubeConfigCluster.Cluster.CertificateAuthorityData,
	)
	if err != nil {
		return err
	}
	c.CAData = string(decodedCa)

	if kubeConfigUser.User.ClientKeyData != "" {
		decodedKey, err := base64.StdEncoding.DecodeString(
			kubeConfigUser.User.ClientKeyData,
		)
		if err != nil {
			return err
		}
		c.KeyData = string(decodedKey)
	}

	if kubeConfigUser.User.ClientCertificateData != "" {
		decodedCert, err := base64.StdEncoding.DecodeString(
			kubeConfigUser.User.ClientCertificateData,
		)
		if err != nil {
			return err
		}
		c.CertData = string(decodedCert)
	}

	c.BearerToken = kubeConfigUser.User.Token
	return nil
}

func getKubeConfigDetails() (*kubeConfigUser, *kubeConfigCluster, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, nil, err
	}
	kubectlConfig, err := readKubeConfig(filepath.Join(usr.HomeDir, ".kube", "config"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read kubeconfig (%w)", err)
	}
	context := extractKubeConfigContext(kubectlConfig, kubectlConfig.CurrentContext)
	if context == nil {
		return nil, nil, fmt.Errorf("failed to find current context in kubeConfig")
	}

	kubeConfigUser := extractKubeConfigUser(kubectlConfig, context.Context.User)
	if kubeConfigUser == nil {
		return nil, nil, fmt.Errorf("failed to find user in kubeConfig")
	}

	kubeConfigCluster := extractKubeConfigCluster(kubectlConfig, context.Context.Cluster)
	if kubeConfigCluster == nil {
		return nil, nil, fmt.Errorf("failed to find cluster in kubeConfig")
	}
	return kubeConfigUser, kubeConfigCluster, nil
}

func extractKubeConfigContext(kubectlConfig kubeConfig, currentContext string) *kubeConfigContext {
	var kubeContext *kubeConfigContext
	for _, ctx := range kubectlConfig.Contexts {
		if ctx.Name == currentContext {
			currentKubeContext := ctx
			kubeContext = &currentKubeContext
			break
		}
	}
	return kubeContext
}

func extractKubeConfigCluster(kubectlConfig kubeConfig, clusterName string) *kubeConfigCluster {
	var kubeConfigCluster *kubeConfigCluster
	for _, c := range kubectlConfig.Clusters {
		if c.Name == clusterName {
			currentKubeConfigCluster := c
			kubeConfigCluster = &currentKubeConfigCluster
			break
		}
	}
	return kubeConfigCluster
}

func extractKubeConfigUser(kubectlConfig kubeConfig, userName string) *kubeConfigUser {
	var kubeConfigUser *kubeConfigUser
	for _, u := range kubectlConfig.Users {
		if u.Name == userName {
			currentConfigUser := u
			kubeConfigUser = &currentConfigUser
			break
		}
	}
	return kubeConfigUser
}

type kubeConfig struct {
	ApiVersion     string              `yaml:"apiVersion" default:"v1"`
	Clusters       []kubeConfigCluster `yaml:"clusters"`
	Contexts       []kubeConfigContext `yaml:"contexts"`
	CurrentContext string              `yaml:"current-context"`
	Kind           string              `yaml:"kind" default:"KubernetesConfig"`
	Preferences    map[string]string   `yaml:"preferences"`
	Users          []kubeConfigUser    `yaml:"users"`
}

type kubeConfigCluster struct {
	Name    string `yaml:"name"`
	Cluster struct {
		CertificateAuthorityData string `yaml:"certificate-authority-testdata"`
		Server                   string `yaml:"server"`
	} `yaml:"cluster"`
}

type kubeConfigContext struct {
	Name    string `yaml:"name"`
	Context struct {
		Cluster string `yaml:"cluster"`
		User    string `yaml:"user"`
	} `yaml:"context"`
}

type kubeConfigUser struct {
	Name string `yaml:"name"`
	User struct {
		ClientCertificateData string `yaml:"client-certificate-testdata"`
		ClientKeyData         string `yaml:"client-key-testdata"`
		Token                 string `yaml:"token"`
	} `yaml:"user"`
}

func readKubeConfig(file string) (config kubeConfig, err error) {
	yamlFile, err := ioutil.ReadFile(file)
	if err != nil {
		return config, err
	}
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return config, err
	}
	return config, nil
}
