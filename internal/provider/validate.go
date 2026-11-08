package provider

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openeverest/openeverest/v2/provider-runtime/controller"

	"github.com/openeverest/provider-kubeai/definition/components"
	"github.com/openeverest/provider-kubeai/definition/topologies/autoscaled"
	"github.com/openeverest/provider-kubeai/internal/common"
)

// Validate checks the rules about the Instance alone. The runtime runs it once
// the Instance matches the topologies, components, versions and parameter
// schemas the Provider declares.
func (p *Provider) Validate(in controller.ValidationInput) controller.ValidationResult {
	var errs field.ErrorList

	if raw := in.Instance.GetTopologyParameters(); raw != nil {
		var topo autoscaled.AutoScaledTopologyParameters
		if json.Unmarshal(raw.Raw, &topo) == nil && topo.MaxReplicas < topo.MinReplicas {
			errs = append(errs, field.Invalid(
				field.NewPath("spec", "topology", "parameters", "maxReplicas"), topo.MaxReplicas,
				fmt.Sprintf("must be at least minReplicas (%d)", topo.MinReplicas),
			))
		}
	}

	serverPath := field.NewPath("spec", "components").Key(common.ComponentServer)
	server, ok := in.Instance.Spec.Components[common.ComponentServer]
	if !ok {
		return controller.ValidationResult{Errors: append(errs, field.Required(serverPath, ""))}
	}

	paramsPath := serverPath.Child("parameters")
	if server.Parameters == nil || server.Parameters.Raw == nil {
		return controller.ValidationResult{Errors: append(errs, field.Required(paramsPath, ""))}
	}
	var params components.VllmParameters
	if err := json.Unmarshal(server.Parameters.Raw, &params); err != nil {
		return controller.ValidationResult{Errors: append(errs, field.Invalid(paramsPath, field.OmitValueType{}, err.Error()))}
	}

	sourcePath := paramsPath.Child("model", "source")
	switch {
	case params.Model.Source == "":
		errs = append(errs, field.Required(sourcePath, ""))
	case engineForSource(params.Model.Source) == "":
		errs = append(errs, field.Invalid(sourcePath, params.Model.Source,
			"must start with hf://, pvc://, ollama://, s3://, gs:// or oss://",
		))
	}

	if err := validateGPUFit(params.Model, gpuQuantity(server)); err != nil {
		errs = append(errs, field.Invalid(
			serverPath.Child("resources", "limits").Key("nvidia.com/gpu"), field.OmitValueType{}, err.Error(),
		))
	}

	return controller.ValidationResult{Errors: errs}
}
