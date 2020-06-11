package controller

import "github.com/tektoncd/operator/pkg/controller/tektonpipeline"

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, tektonpipeline.Add)
}
