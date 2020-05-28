package cli

import (
	"context"
	"os"
	"time"

	cli "github.com/urfave/cli/v2"
	"google.golang.org/grpc/metadata"
)

// ServerFlags ..
type ServerFlags struct {
	CompanyName          string
	ProjectName          string
	ServerName           string
	ServerEnv            string
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
	ConfigType           string
	ConfigHost           string
	ServerDomain         string

	ServiceName string
}

var (
	serverFlags = &ServerFlags{}
	// DefaultServerFlags default server flag ..
	DefaultServerFlags = []cli.Flag{
		&cli.StringFlag{
			Name:    "company_name",
			Value:   "",
			EnvVars: []string{"COMPANY_NAME"},
			Usage:   "Name of the company. company-project-service",
		},
		&cli.StringFlag{
			Name:    "project_name",
			EnvVars: []string{"MICRO_PROJECT_NAME"},
			Usage:   "Name of the project. company-project-service",
		},
		// &cli.StringFlag{
		// 	Name:    "client",
		// 	EnvVars: []string{"MICRO_CLIENT"},
		// 	Usage:   "Client for go-micro; rpc",
		// },
		&cli.StringFlag{
			Name:    "client_request_timeout",
			EnvVars: []string{"MICRO_CLIENT_REQUEST_TIMEOUT"},
			Value:   "5s",
			Usage:   "Sets the client request timeout. e.g 500ms, 5s, 1m. Default: 5s",
		},
		&cli.IntFlag{
			Name:    "client_retries",
			EnvVars: []string{"MICRO_CLIENT_RETRIES"},
			Value:   1,
			Usage:   "Sets the client retries. Default: 1",
		},
		// &cli.IntFlag{
		// 	Name:    "client_pool_size",
		// 	EnvVars: []string{"MICRO_CLIENT_POOL_SIZE"},
		// 	Usage:   "Sets the client connection pool size. Default: 1",
		// },
		// &cli.StringFlag{
		// 	Name:    "client_pool_ttl",
		// 	EnvVars: []string{"MICRO_CLIENT_POOL_TTL"},
		// 	Usage:   "Sets the client connection pool ttl. e.g 500ms, 5s, 1m. Default: 1m",
		// },
		&cli.IntFlag{
			Name:    "register_ttl",
			EnvVars: []string{"MICRO_REGISTER_TTL"},
			Value:   60,
			Usage:   "Register TTL in seconds",
		},
		&cli.IntFlag{
			Name:    "register_interval",
			EnvVars: []string{"MICRO_REGISTER_INTERVAL"},
			Value:   30,
			Usage:   "Register interval in seconds",
		},
		// &cli.StringFlag{
		// 	Name:    "server",
		// 	EnvVars: []string{"MICRO_SERVER"},
		// 	Usage:   "Server for go-micro; rpc",
		// },
		&cli.StringFlag{
			Name:    "server_name",
			EnvVars: []string{"MICRO_SERVER_NAME"},
			Usage:   "Name of the server. go.micro.srv.example",
		},
		&cli.StringFlag{
			Name:    "server_env",
			EnvVars: []string{"SERVER_ENV"},
			Usage:   "env of the server. release,staging,dev,private",
		},
		&cli.StringFlag{
			Name:    "server_version",
			EnvVars: []string{"MICRO_SERVER_VERSION"},
			Usage:   "Version of the server. 1.1.0",
		},
		&cli.StringFlag{
			Name:    "server_id",
			EnvVars: []string{"MICRO_SERVER_ID"},
			Usage:   "Id of the server. Auto-generated if not specified",
		},
		&cli.IntFlag{
			Name:    "replica_id",
			EnvVars: []string{"MICRO_REPLICA_ID"},
			Usage:   "Id of the replica. default 0",
		},
		&cli.StringFlag{
			Name:    "server_address",
			EnvVars: []string{"MICRO_SERVER_ADDRESS"},
			Usage:   "Bind address for the server. 127.0.0.1:8080",
		},
		&cli.StringFlag{
			Name:    "server_advertise",
			EnvVars: []string{"MICRO_SERVER_ADVERTISE"},
			Usage:   "Used instead of the server_address when registering with discovery. 127.0.0.1:8080",
		},
		// &cli.StringSliceFlag{
		// 	Name:    "server_metadata",
		// 	EnvVars: []string{"MICRO_SERVER_METADATA"},
		// 	Value:   &cli.StringSlice{},
		// 	Usage:   "A list of key-value pairs defining metadata. version=1.0.0",
		// },
		&cli.StringFlag{
			Name:    "broker",
			EnvVars: []string{"MICRO_BROKER"},
			Usage:   "Broker for pub/sub. http, nats, rabbitmq",
		},
		&cli.StringFlag{
			Name:    "broker_address",
			EnvVars: []string{"MICRO_BROKER_ADDRESS"},
			Usage:   "Comma-separated list of broker addresses",
		},
		&cli.StringFlag{
			Name:    "registry",
			EnvVars: []string{"MICRO_REGISTRY"},
			Usage:   "Registry for discovery. consul, etcd, mdns",
		},
		&cli.StringFlag{
			Name:    "registry_address",
			EnvVars: []string{"MICRO_REGISTRY_ADDRESS"},
			Usage:   "Comma-separated list of registry addresses",
		},
		// &cli.StringFlag{
		// 	Name:    "selector",
		// 	EnvVars: []string{"MICRO_SELECTOR"},
		// 	Usage:   "Selector used to pick nodes for querying",
		// },
		&cli.StringFlag{
			Name:    "transport",
			EnvVars: []string{"MICRO_TRANSPORT"},
			Usage:   "Transport mechanism used; http",
		},
		&cli.StringFlag{
			Name:    "transport_address",
			EnvVars: []string{"MICRO_TRANSPORT_ADDRESS"},
			Usage:   "Comma-separated list of transport addresses",
		},
		&cli.StringFlag{
			Name:    "config_address",
			Aliases: []string{"c"},
			EnvVars: []string{"CONFIG_ADDRESS"},
			Usage:   "Address of config",
		},
		&cli.StringFlag{
			Name:    "config_type",
			EnvVars: []string{"CONFIG_TYPE"},
			Usage:   "type of config",
		},
		&cli.StringFlag{
			Name:    "config_host",
			EnvVars: []string{"CONFIG_HOST"},
			Usage:   "host of config",
		},
		&cli.StringFlag{
			Name:    "server_domain",
			EnvVars: []string{"SERVER_DOMAIN"},
			Usage:   "The domain of server",
		},
	}
)

// Init ..
func Init() error {
	app := cli.NewApp()
	app.UseShortOptionHandling = true
	app.Flags = DefaultServerFlags
	app.Action = func(c *cli.Context) error {
		serverFlags = &ServerFlags{
			CompanyName:      c.String("company_name"),
			ProjectName:      c.String("project_name"),
			ServerName:       c.String("server_name"),
			ServerEnv:        c.String("server_env"),
			ServerVersion:    c.String("server_version"),
			ServerID:         c.String("server_id"),
			ReplicaID:        c.Int("replica_id"),
			ServerAddress:    c.String("server_address"),
			ServerAdvertise:  c.String("server_advertise"),
			ClientRetries:    c.Int("client_retries"),
			RegisterTTL:      time.Duration(c.Int("register_ttl")) * time.Second,
			RegisterInternal: time.Duration(c.Int("register_interval")) * time.Second,
			Broker:           c.String("broker"),
			BrokerAddress:    c.String("broker_address"),
			Registry:         c.String("registry"),
			RegistryAddress:  c.String("registry_address"),
			ConfigAddress:    c.String("config_address"),
			ConfigType:       c.String("config_type"),
			ConfigHost:       c.String("config_host"),
			ServerDomain:     c.String("server_domain"),
		}
		serverFlags.ServiceName = FormatStrings([]string{
			serverFlags.CompanyName,
			serverFlags.ProjectName,
			serverFlags.ServerName})

		if c.String("client_request_timeout") != "" {
			var err error
			serverFlags.ClientRequestTimeout, err =
				time.ParseDuration(c.String("client_request_timeout"))
			if err != nil {
				return err
			}
		}
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		return err
	}
	return nil
}

// ServiceTopic ..
func ServiceTopic(str []string) string {
	return FormatStrings(str)
}

// ServiceName ..
func ServiceName(str []string) string {
	return FormatStrings(str)
}

// GetServiceName ..
func GetServiceName() string {
	return serverFlags.ServiceName
}

// GetServicePrefix ..
func GetServicePrefix() string {
	if serverFlags.ServiceName != "" {
		return serverFlags.ServiceName + "-"
	}
	return ""
}

// GetCompanyName ..
func GetCompanyName() string {
	return serverFlags.CompanyName
}

// GetProjectName ..
func GetProjectName() string {
	return serverFlags.ProjectName
}

// GetServerName ..
func GetServerName() string {
	return serverFlags.ServerName
}

// GetServerID ..
func GetServerID() string {
	return serverFlags.ServerID
}

// GetServerVersion ..
func GetServerVersion() string {
	return serverFlags.ServerVersion
}

// GetServerAddress ..
func GetServerAddress() string {
	return serverFlags.ServerAddress
}

// GetServerAdvertise ..
func GetServerAdvertise() string {
	return serverFlags.ServerAdvertise
}

// GetBrokerAddress ..
func GetBrokerAddress() string {
	return serverFlags.BrokerAddress
}

// GetRegistryAddress ..
func GetRegistryAddress() string {
	return serverFlags.RegistryAddress
}

// GetConfigAddress ..
func GetConfigAddress() string {
	return serverFlags.ConfigAddress
}

// GetConfigType ..
func GetConfigType() string {
	return serverFlags.ConfigType
}

// GetConfigHost ..
func GetConfigHost() string {
	return serverFlags.ConfigHost
}

// GetReplicaID ..
func GetReplicaID() int {
	return serverFlags.ReplicaID
}

// GetClientRetries ..
func GetClientRetries() int {
	return serverFlags.ClientRetries
}

// GetClientRequestTimeout ..
func GetClientRequestTimeout() time.Duration {
	return serverFlags.ClientRequestTimeout
}

// GetRegisterTTL ..
func GetRegisterTTL() time.Duration {
	return serverFlags.RegisterTTL
}

// GetRegisterInternal ..
func GetRegisterInternal() time.Duration {
	return serverFlags.RegisterInternal
}

// GetServerDomain ..
func GetServerDomain() string {
	return serverFlags.ServerDomain
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

// WithServiceContext ..
func WithServiceContext(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "service", GetServiceName())
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
