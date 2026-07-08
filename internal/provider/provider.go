package provider

import (
	"fmt"

	kubeaiv1 "github.com/kubeai-project/kubeai/api/k8s/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/openeverest/openeverest/v2/provider-runtime/controller"
	"github.com/openeverest/provider-kubeai/definition"
	"github.com/openeverest/provider-kubeai/definition/components"
	"github.com/openeverest/provider-kubeai/definition/topologies/autoscaled"
	"github.com/openeverest/provider-kubeai/internal/common"
)

// Compile-time check that Provider implements the required interface.
var _ controller.ProviderInterface = (*Provider)(nil)

// Provider implements controller.ProviderInterface for the provider-kubeai provider
// This transaltes OpenEverest Instance CRs into KubeAI Model CRs
type Provider struct {
	controller.BaseProvider
}

func New() *Provider {
	return &Provider{
		BaseProvider: controller.BaseProvider{
			ProviderName: common.ProviderName,
			SchemeFuncs: []func(*runtime.Scheme) error {
				kubeaiv1.SchemeBuilder.AddToScheme,
			} ,
			WatchConfigs: []controller.WatchConfig{
				controller.WatchOwned(&kubeaiv1.Model{}),
			},
		},
	}
}

// Validate checks if the Instance spec is valid before the Sync runs
func (p *Provider) Validate(c *controller.Context) error {
	srv, ok := c.Instance().Spec.Components[common.ComponentServer]
	if !ok {
		return fmt.Errorf("component.%s is required", common.ComponentServer)
	}
	
	var cs components.VllmCustomSpec
	if err := c.DecodeComponentCustomSpec(
		srv,
		&cs,
	);
	err != nil {
		return fmt.Errorf("components.%s.customSpec is required : %w", common.ComponentServer, err)
	}

	if cs.Model.Source == "" {
		return fmt.Errorf("components.%s.customSpec.model.source is required", common.ComponentServer)
	}

	// todo engineForSource
	
	// AutoScaled Topology validator for min and max replicas count
	var topo autoscaled.AutoScaledTopologyConfig
	if c.TryDecodeTopologyConfig(&topo) {
		if topo.MaxReplicas < topo.MinReplicas {
			return fmt.Errorf("topology.config.maxReplicas (%d) must be >= minReplicas (%d)",
				topo.MaxReplicas, topo.MinReplicas)
		}
	}

	return // validateGPUfit

}

// Sync builds the KubeAI Model CR from the Instance spec and applies it
// c.Apply sets the owner reference , so the Model is garbage collected with the Instance
// func (p *Provider) Sync(c *controller.Context) error {
// 	l := log.FromContext(c.Context())

// 	srv := c.Instance().Spec.Components[common.ComponentServer]

// 	var cs components.VllmCustomSpec
// 	_ = c.TryDecodeComponentCustomSpec(srv, &cs)
	
// 	top := autoscaled.AutoScaledTopologyConfig{
// 		MinReplicas: 0,
// 		MaxReplicas: 1,
// 	}
// 	_ = c.TryDecodeTopologyConfig(&top)

// 	var global definition.GlobalConfig
// 	_ = c.DecodeGlobalConfig(&global)

// 	model := &kubeaiv1.Model{
// 		ObjectMeta: c.ObjectMeta(c.Name()),
// 		Spec: kubeaiv1.ModelSpec{
// 			URL: cs.Model.Source,
// 			Engine: ,
// 			Features
// 		},
// 	}

// 	l.Info("Syncing KubeAI Model", "name", model.Name, "url", model.Spec.URL, "engine", model.Spec.Engine)
// 	return c.Apply(model)
// }

// Status reads the KubeAI Model status and maps it to an Instance phase
func (p *Provider) Status(c *controller.Context) (controller.Status, error) {
	model := &kubeaiv1.Model{}
	if err := c.Get(model, c.Name());
	err != nil {
		if controller.IsNotFound(err) {
			return controller.Provisioning("waiting for KubeAI model to be created"), nil
		}
		return controller.Status{}, err
	}

	switch {
	case model.Status.Replicas.Ready > 0:
		return controller.ReadyWithConnectionDetails(connectionDetails(c)), nil
	case model.Status.Cache != nil && !model.Status.Cache.Loaded:
		return controller.Initializing("downloading model weights into cache"), nil
	case model.Status.Replicas.All == 0:
		// minReplicas=0 and no in-flight requests then compute is scale down to zero
		return controller.Suspended(), nil
	default:
		return controller.Provisioning("waiting for model server pods to become ready"), nil
	}
}


// Helper functions

// connectionDetails points clients at the shared KubeAI compatible proxy
// the model is addressed by name in the request payload
func connectionDetails(c *controller.Context) controller.ConnectionDetails {
	return controller.ConnectionDetails{
		Type: "llm",
		Provider: common.ProviderName,
		Host: "kubeai." + c.Namespace() + ".svc.cluster.local",
		Port: "80",
		URI: fmt.Sprintf("http://kubeai.%s.svc.cluster.local/openai/v1", c.Namespace()),
		AdditionalProperties: map[string]string{
			"modelName" : c.Name(),
			"basePath" : "/openai/v1",
		},
	}
}