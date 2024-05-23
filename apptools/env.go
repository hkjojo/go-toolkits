package apptools

import (
	"errors"
	"io"
	"os"

	"github.com/urfave/cli/v2"
)

// go build -ldflags "-X package.Version=x.y.z"
var (
	// ...
	ConfigFilePath string
	// Name is the name of the compiled software.
	Name = "acttrade.service.basic"
	// Version is the version of the compiled software.
	Version string
	// Tag is for identify different envs like demo/live.
	Tag string
	// Env is used to specify the running environment.
	Env string

	// node name from k8s cluster
	ClusterNodeName string
	// pod name from k8s cluster
	ClusterPodName string
	// Datadog agent host
	DDAgentHost string
	ID, _       = os.Hostname()

	EmptyApp = &cli.App{}

	// options and changable, only for mapping mt5/mt4 demo/live
	TagFlag = &cli.StringFlag{
		Name:        "service_tag",
		Aliases:     []string{"tag"},
		Usage:       "eg: -tag [demo|live|test]",
		EnvVars:     []string{"SERVICE_TAG"},
		Required:    true,
		Destination: &Tag,
	}
)

func NewDefaultApp() *cli.App {
	mainApp := cli.NewApp()
	mainApp.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "environment",
			Aliases:     []string{"env"},
			DefaultText: "dev",
			Usage:       "eg: -env prod|beta|dev",
			EnvVars:     []string{"SERVICE_ENV"},
			Destination: &Env,
		},
		&cli.StringFlag{
			Name:        "service_name",
			Aliases:     []string{"name"},
			DefaultText: Name,
			Usage:       "eg: -name {project}.service.{srv}",
			EnvVars:     []string{"SERVICE_NAME"},
		},
		&cli.StringFlag{
			Name:        "conf",
			Aliases:     []string{"c"},
			DefaultText: "../../configs",
			Usage:       "eg: -conf config.yaml",
			EnvVars:     []string{"CONFIG_PATH"},
			Destination: &ConfigFilePath,
		},
		&cli.StringFlag{
			Name:        "cluster_node_name",
			Aliases:     []string{"cnn"},
			EnvVars:     []string{"CLUSTER_NODE_NAME"},
			Destination: &ClusterNodeName,
		},
		&cli.StringFlag{
			Name:        "cluster_pod_name",
			Aliases:     []string{"cpn"},
			EnvVars:     []string{"CLUSTER_POD_NAME"},
			Destination: &ClusterPodName,
		},
		&cli.StringFlag{
			Name:        "dd_agent_host",
			EnvVars:     []string{"DD_AGENT_HOST"},
			Destination: &DDAgentHost,
		},
	}

	mainApp.Action = func(c *cli.Context) error {
		if name := c.String("service_name"); name != "" {
			Name = name
		}

		tag := c.String("service_tag")
		if tag != "" && tag != "demo" && tag != "live" && tag != "test" {
			return errors.New("invalid tag value")
		}
		return nil
	}

	oldHelpPrinter := cli.HelpPrinter
	cli.HelpPrinter = func(w io.Writer, templ string, data interface{}) {
		oldHelpPrinter(w, templ, data)
		os.Exit(0)
	}
	return mainApp
}
