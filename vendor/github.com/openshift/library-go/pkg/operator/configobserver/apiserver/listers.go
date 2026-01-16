package apiserver

import (
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
)

type APIServerLister interface {
	APIServerLister() configlistersv1.APIServerLister
}
