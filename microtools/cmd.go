package microtools

import (
	"context"
	"time"

	"github.com/micro/cli/v2"
	"github.com/micro/go-micro/v2/config/cmd"
	mmetadata "github.com/micro/go-micro/v2/metadata"
	"google.golang.org/grpc/metadata"
)

// CmdOptions ..
type CmdOptions struct {
	CompanyName          string
	ProjectName          string
	ServerName           string
	ServerID             string
	ReplicaID            int
	ClientRetries        int
	ClientRequestTimeout time.Duration
	RegisterTTL          time.Duration
	RegisterInternal     time.Duration
	ServerVersion        string
	ServerAddress        string
	ServerAdvertise      string
	Broker               string
	BrokerAddress        string
	Registry             string
	RegistryAddress      string
	ConfigAddress        string

	ServiceName string
}

var (
	options = &CmdOptions{}
)

// InitCmd ...
func InitCmd() error {
	app := cmd.App()
	app.Flags = append(app.Flags,
		&cli.StringFlag{
			Name:    "company_name",
			EnvVars: []string{"MICRO_COMPANY_NAME"},
			Usage:   "Company of the project. company.project.service",
		},
		&cli.StringFlag{
			Name:    "project_name",
			EnvVars: []string{"MICRO_PROJECT_NAME"},
			Usage:   "Name of the project. project.service",
		},
		&cli.IntFlag{
			Name:    "replica_id",
			EnvVars: []string{"MICRO_REPLICA_ID"},
			Usage:   "ID of the replica.",
		},
		&cli.StringFlag{
			Name:    "config_address",
			EnvVars: []string{"MICRO_CONFIG_ADDRESS"},
			Usage:   "Address of the config.",
		})

	before := app.Before
	app.Before = func(ctx *cli.Context) error {
		options = &CmdOptions{
			CompanyName:      ctx.String("company_name"),
			ProjectName:      ctx.String("project_name"),
			ServerName:       ctx.String("server_name"),
			ServerID:         ctx.String("server_id"),
			ServerVersion:    ctx.String("server_version"),
			ServerAddress:    ctx.String("server_address"),
			ServerAdvertise:  ctx.String("server_advertise"),
			ClientRetries:    ctx.Int("client_retries"),
			RegisterTTL:      time.Duration(ctx.Int("register_ttl")) * time.Second,
			RegisterInternal: time.Duration(ctx.Int("register_interval")) * time.Second,
			Broker:           ctx.String("broker"),
			BrokerAddress:    ctx.String("broker_address"),
			Registry:         ctx.String("registry"),
			RegistryAddress:  ctx.String("registry_address"),
			ReplicaID:        ctx.Int("replica_id"),
			ConfigAddress:    ctx.String("config_address"),
		}

		options.ServiceName = FormatStrings([]string{
			options.CompanyName,
			options.ProjectName,
			options.ServerName})

		if ctx.String("client_request_timeout") != "" {
			var err error
			options.ClientRequestTimeout, err =
				time.ParseDuration(ctx.String("client_request_timeout"))
			if err != nil {
				return err
			}
		}

		return before(ctx)
	}

	return cmd.Init()
}

// ServiceTopic ..
func ServiceTopic(str []string) string {
	return FormatStrings(str)
}

// ServiceName ..
func ServiceName(str []string) string {
	return FormatStrings(str)
}

// WithServiceContext ..
func WithServiceContext(ctx context.Context) context.Context {
	return mmetadata.Set(ctx, "service", GetServiceName())
}

// GetContextService get service name from context
func GetContextService(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	vs := md.Get("service")
	if len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// Prefix ..
func Prefix(strs []string) (result string) {
	result = FormatStrings(strs)
	if result != "" {
		result = result + "-"
	}
	return
}

// FormatStrings ..
func FormatStrings(strs []string) string {
	var str string
	for _, s := range strs {
		if s == "" {
			continue
		}
		if str == "" {
			str = s
			continue
		}
		str = str + "-" + s
	}
	return str
}

// GetServiceName ..
func GetServiceName() string {
	return options.ServiceName
}

// GetServicePrefix ..
func GetServicePrefix() string {
	if options.ServiceName != "" {
		return options.ServiceName + "-"
	}
	return ""
}

// GetCompanyName ..
func GetCompanyName() string {
	return options.CompanyName
}

// GetProjectName ..
func GetProjectName() string {
	return options.ProjectName
}

// GetServerName ..
func GetServerName() string {
	return options.ServerName
}

// GetServerID ..
func GetServerID() string {
	return options.ServerID
}

// GetServerVersion ..
func GetServerVersion() string {
	return options.ServerVersion
}

// GetServerAddress ..
func GetServerAddress() string {
	return options.ServerAddress
}

// GetServerAdvertise ..
func GetServerAdvertise() string {
	return options.ServerAdvertise
}

// GetBrokerAddress ..
func GetBrokerAddress() string {
	return options.BrokerAddress
}

// GetRegistryAddress ..
func GetRegistryAddress() string {
	return options.RegistryAddress
}

// GetConfigAddress ..
func GetConfigAddress() string {
	return options.ConfigAddress
}

// GetReplicaID ..
func GetReplicaID() int {
	return options.ReplicaID
}

// GetClientRetries ..
func GetClientRetries() int {
	return options.ClientRetries
}

// GetClientRequestTimeout ..
func GetClientRequestTimeout() time.Duration {
	return options.ClientRequestTimeout
}

// GetRegisterTTL ..
func GetRegisterTTL() time.Duration {
	return options.RegisterTTL
}

// GetRegisterInternal ..
func GetRegisterInternal() time.Duration {
	return options.RegisterInternal
}

// SetOptions ...
func SetOptions(f func(*CmdOptions)) {
	f(options)
}
