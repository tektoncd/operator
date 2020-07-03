package manifestival

import "github.com/go-logr/logr"

type Option func(*Manifest)

func UseLogger(log logr.Logger) Option {
	return func(m *Manifest) {
		m.log = log
	}
}

func UseClient(client Client) Option {
	return func(m *Manifest) {
		m.Client = client
	}
}
