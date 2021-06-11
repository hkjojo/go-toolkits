package kafka

import (
	"errors"

	"github.com/Shopify/sarama"
)

// errors defined
var (
	ErrAlreadyClosed = errors.New("producer already closed")
)

type Option func(*sarama.Config)
