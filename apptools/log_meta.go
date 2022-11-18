package apptools

import (
	"github.com/go-kratos/kratos/v2/log"
)

const (
	MetaKey_ENV      = "env"
	MetaKey_HOSTNAME = "hostname"
	MetaKey_SERVICE  = "service"
	MetaKey_VERSION  = "version"
	MetaKey_INSTANCE = "instance"
	MetaKey_CALLER   = "caller"
)

func WithMetaKeys(logger log.Logger) log.Logger {
	kvs := []interface{}{
		MetaKey_CALLER, log.Caller(5),
	}
	if Name != "" {
		kvs = append(kvs, MetaKey_SERVICE, Name)
	}
	if Version != "" {
		kvs = append(kvs, MetaKey_VERSION, Version)
	}
	if Env != "" {
		kvs = append(kvs, MetaKey_ENV, Env)
	}
	if ClusterNodeName != "" {
		kvs = append(kvs, MetaKey_HOSTNAME, ClusterNodeName)
	}

	if ClusterPodName != "" {
		kvs = append(kvs, MetaKey_INSTANCE, ClusterPodName)
	}
	return log.With(logger, kvs...)
}
