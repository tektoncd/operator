package main

import (
	"fmt"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"sigs.k8s.io/yaml"
)

func main() {
	scheduler := v1alpha1.Scheduler{}
	//logger := logging.FromContext(ctx)
	scheduler.SetDefaults()

	pruner := v1alpha1.Pruner{}
	pruner.SetDefaults()

	data, _ := yaml.Marshal(scheduler)
	fmt.Println(string(data))

	//data, _ = yaml.Marshal(pruner)
	//fmt.Println(string(data))

}
