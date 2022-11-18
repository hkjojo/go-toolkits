package apptools

import (
	"os"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
)

type (
	NewSourceFunc             func() config.Source
	NewOtelTracerProviderFunc func() (*trace.TracerProvider, func(), error)
	NewLogFunc                func() (log.Logger, error)
	NewAppFunc                func(*App) (func(), error)
)

type Builder struct {
	app           *cli.App
	cfgs          []interface{}
	tpFactory     NewOtelTracerProviderFunc
	logFactory    NewLogFunc
	sourceFactory NewSourceFunc
	funcs         []NewAppFunc
}

type App struct {
	logger log.Logger
	tp     *trace.TracerProvider
	source config.Config
}

func (a *App) Logger() log.Logger {
	return a.logger
}

func (a *App) Source() config.Config {
	return a.source
}

// NewBuilder ...
func NewBuilder() *Builder {
	return &Builder{
		app: NewDefaultApp(),
	}
}

func (b *Builder) AddOtelTraceProvider(f NewOtelTracerProviderFunc) *Builder {
	b.tpFactory = f
	return b
}

func (b *Builder) AddFlags(flags ...cli.Flag) *Builder {
	b.app.Flags = append(b.app.Flags, flags...)
	return b
}

func (b *Builder) AddAction(action cli.ActionFunc) *Builder {
	originAction := b.app.Action
	// merge actions
	b.app.Action = func(c *cli.Context) error {
		err := originAction(c)
		if err != nil {
			return err
		}
		return action(c)
	}
	return b
}

func (b *Builder) AddLogFactory(f NewLogFunc) *Builder {
	b.logFactory = f
	return b
}

func (b *Builder) AddSource(f NewSourceFunc) *Builder {
	b.sourceFactory = f
	return b
}

func (b *Builder) AddConfigs(cfgs ...interface{}) *Builder {
	b.cfgs = append(b.cfgs, cfgs...)
	return b
}

func (b *Builder) AddFunc(f NewAppFunc) *Builder {
	b.funcs = append(b.funcs, f)
	return b
}

// Build return cleanup function and error
func (b *Builder) Build() (*App, func(), error) {
	app := &App{}
	// init env flags
	err := b.app.Run(os.Args)
	if err != nil {
		return nil, nil, err
	}

	// init config
	if b.sourceFactory != nil {
		app.source = config.New(config.WithSource(b.sourceFactory()))
		defer app.source.Close()
		err = app.source.Load()
		if err != nil {
			return nil, nil, err
		}

		for _, cfg := range b.cfgs {
			err = app.source.Scan(cfg)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	// init logger
	if b.logFactory != nil {
		logger, err := b.logFactory()
		if err != nil {
			return nil, nil, err
		}

		app.logger = WithMetaKeys(logger)
	}

	var cleanups []func()
	// init trace provider
	if b.tpFactory != nil {
		tp, cleanup, err := b.tpFactory()
		if err != nil {
			return nil, nil, err
		}
		app.tp = tp
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{}))
		otel.SetTracerProvider(app.tp)
		cleanups = append(cleanups, cleanup)
	}

	// init funcs
	for _, f := range b.funcs {
		cleanup, err := f(app)
		if err != nil {
			return nil, nil, err
		}
		cleanups = append(cleanups, cleanup)
	}

	return app, func() {
		l := len(cleanups) - 1
		for i := l; i >= 0; i-- {
			cleanups[i]()
		}
	}, nil
}
