package provider

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeaiv1 "github.com/kubeai-project/kubeai/api/k8s/v1"
	corev1alpha1 "github.com/openeverest/openeverest/v2/api/core/v1alpha1"
	"github.com/openeverest/openeverest/v2/provider-runtime/controller"

	"github.com/openeverest/provider-kubeai/definition"
	"github.com/openeverest/provider-kubeai/definition/components"
	"github.com/openeverest/provider-kubeai/definition/topologies/autoscaled"
	"github.com/openeverest/provider-kubeai/internal/common"
)

// Compile-time check that Provider implements the required interface.
var _ controller.ProviderInterface = (*Provider)(nil)

// Provider implements controller.ProviderInterface for the provider-kubeai provider
// This translates OpenEverest Instance CRs into KubeAI Model CRs
type Provider struct {
	controller.BaseProvider
}

// New constructs the KubeAI provider with the runtime metadata needed by the
// shared controller.
func New() *Provider {
	return &Provider{
		BaseProvider: controller.BaseProvider{
			// ProviderName identifies this provider in OpenEverest status and connection details.
			ProviderName: common.ProviderName,
			// SchemeFuncs registers KubeAI resources with the controller-runtime scheme.
			SchemeFuncs: []func(*runtime.Scheme) error{
				kubeaiv1.SchemeBuilder.AddToScheme,
			},
			// WatchConfigs tells the provider to reconcile Instances when owned Models change.
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

	var cs components.VllmParameters
	if err := c.DecodeComponentParameters(
		srv,
		&cs,
	); err != nil {
		return fmt.Errorf("components.%s.parameters is required : %w", common.ComponentServer, err)
	}

	if cs.Model.Source == "" {
		return fmt.Errorf("components.%s.parameters.model.source is required", common.ComponentServer)
	}

	if engineForSource(cs.Model.Source) == "" {
		return fmt.Errorf("model.source must start with hf://, pvc://, ollama://, s3://, gs:// or oss://")
	}

	// AutoScaled Topology validator for min and max replicas count
	var topo autoscaled.AutoScaledTopologyParameters
	if c.TryDecodeTopologyParameters(&topo) {
		if topo.MaxReplicas < topo.MinReplicas {
			return fmt.Errorf("topology.parameters.maxReplicas (%d) must be >= minReplicas (%d)",
				topo.MaxReplicas, topo.MinReplicas)
		}
	}

	return validateGPUFit(cs.Model, gpuQuantity(srv))

}

// Sync ensures the KubeAI Model CR matches the Instance desired state.
//
// It uses CreateOrUpdate and only writes fields this provider owns. It never
// touches Spec.Replicas — KubeAI's autoscaler owns that for scale-from-zero.
// When desired fields are already correct, CreateOrUpdate is a no-op so we do
// not bump generation or fight the model controller.
func (p *Provider) Sync(c *controller.Context) error {
	l := log.FromContext(c.Context())

	srv := c.Instance().Spec.Components[common.ComponentServer]

	var cs components.VllmParameters
	_ = c.TryDecodeComponentParameters(srv, &cs)

	top := autoscaled.AutoScaledTopologyParameters{
		MinReplicas: 0,
		MaxReplicas: 1,
	}
	_ = c.TryDecodeTopologyParameters(&top)

	var params definition.Parameters
	_ = c.TryDecodeParameters(&params)

	desiredURL := cs.Model.Source
	desiredEngine := engineForSource(cs.Model.Source)
	desiredFeatures := featuresForTask(params.Task)
	desiredProfile := resourceProfile(cs, srv)
	desiredMax := top.MaxReplicas

	model := &kubeaiv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name(),
			Namespace: c.Namespace(),
		},
	}

	op, err := controllerutil.CreateOrUpdate(c.Context(), c.Client(), model, func() error {
		if model.Labels == nil {
			model.Labels = map[string]string{}
		}
		model.Labels["app.kubernetes.io/managed-by"] = "everest"
		model.Labels["app.kubernetes.io/instance"] = c.Name()

		model.Spec.URL = desiredURL
		model.Spec.Engine = desiredEngine
		model.Spec.Features = desiredFeatures
		model.Spec.ResourceProfile = desiredProfile
		model.Spec.CacheProfile = cs.CacheProfile
		model.Spec.Args = cs.Args
		model.Spec.Env = cs.Env
		model.Spec.MinReplicas = top.MinReplicas
		model.Spec.MaxReplicas = &desiredMax
		if top.TargetRequests != nil {
			model.Spec.TargetRequests = top.TargetRequests
		}
		if top.ScaleDownDelaySeconds != nil {
			model.Spec.ScaleDownDelaySeconds = top.ScaleDownDelaySeconds
		}
		// Spec.Replicas is owned by KubeAI — do not set or clear it.

		return controllerutil.SetControllerReference(c.Instance(), model, c.Client().Scheme())
	})
	if err != nil {
		return fmt.Errorf("sync KubeAI Model %q: %w", c.Name(), err)
	}

	l.Info("Synced KubeAI Model", "name", model.Name, "op", op, "url", desiredURL, "engine", desiredEngine)
	return nil
}

// Status reads the KubeAI Model status and maps it to an Instance phase
func (p *Provider) Status(c *controller.Context) (controller.Status, error) {
	model := &kubeaiv1.Model{}
	if err := c.Get(model, c.Name()); err != nil {
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

// Cleanup handles deletion. The Model CR carries an owner reference set in
// Sync, so Kubernetes garbage collection removes it with the Instance.
func (p *Provider) Cleanup(c *controller.Context) error {
	return nil
}

// =============================================================================
// Helpers
// =============================================================================

// connectionDetails points clients at the shared KubeAI compatible proxy
// the model is addressed by name in the request payload
func connectionDetails(c *controller.Context) controller.ConnectionDetails {
	return controller.ConnectionDetails{
		Type:     "llm",
		Provider: common.ProviderName,
		Host:     "kubeai." + c.Namespace() + ".svc.cluster.local",
		Port:     "80",
		URI:      fmt.Sprintf("http://kubeai.%s.svc.cluster.local/openai/v1", c.Namespace()),
		AdditionalProperties: map[string]string{
			"modelName": c.Name(),
			"basePath":  "/openai/v1",
		},
	}
}

// engineForSource picks the KubeAI engine from the model source scheme.
// Everything except ollama:// is served by vLLM in this provider.
func engineForSource(source string) string {
	switch {
	case strings.HasPrefix(source, "ollama://"):
		return kubeaiv1.OLlamaEngine
	case strings.HasPrefix(source, "hf://"),
		strings.HasPrefix(source, "pvc://"),
		strings.HasPrefix(source, "s3://"),
		strings.HasPrefix(source, "gs://"),
		strings.HasPrefix(source, "oss://"):
		return kubeaiv1.VLLMEngine
	default:
		return ""
	}
}

// featuresForTask maps the instance-wide task parameter to KubeAI model features.
func featuresForTask(task string) []kubeaiv1.ModelFeature {
	switch task {
	case "TextEmbedding":
		return []kubeaiv1.ModelFeature{kubeaiv1.ModelFeatureTextEmbedding}
	case "SpeechToText":
		return []kubeaiv1.ModelFeature{kubeaiv1.ModelFeatureSpeechToText}
	default:
		return []kubeaiv1.ModelFeature{kubeaiv1.ModelFeatureTextGeneration}
	}
}

// resourceProfile returns the KubeAI resource profile for the server.
// An explicit parameters.resourceProfile wins; otherwise it is derived from
// the GPU count in the component resources ("cpu:1" when no GPU is requested).
func resourceProfile(cs components.VllmParameters, srv corev1alpha1.ComponentSpec) string {
	if cs.ResourceProfile != "" {
		return cs.ResourceProfile
	}
	if gpus := gpuQuantity(srv); gpus > 0 {
		return fmt.Sprintf("nvidia-gpu-l4:%d", gpus)
	}
	return "cpu:1"
}

// gpuQuantity returns the number of nvidia.com/gpu requested in limits.
func gpuQuantity(srv corev1alpha1.ComponentSpec) int64 {
	if srv.Resources == nil {
		return 0
	}
	q, ok := srv.Resources.Limits[corev1.ResourceName("nvidia.com/gpu")]
	if !ok {
		return 0
	}
	return q.Value()
}

// bytesPerParam maps quantization to approximate bytes per model parameter.
var bytesPerParam = map[string]float64{
	"fp16": 2.0,
	"int8": 1.0,
	"int4": 0.5,
}

// validateGPUFit is a coarse heuristic: weights must fit in the aggregate GPU
// memory (assuming ~24Gi-class GPUs such as L4) with 20% headroom for KV cache.
// It only runs when the user supplies estimatedParamBillions.
func validateGPUFit(m components.ModelSpec, gpus int64) error {
	if m.EstimatedParamBillions == nil || gpus == 0 {
		return nil
	}
	bpp, ok := bytesPerParam[m.Quantization]
	if !ok {
		bpp = bytesPerParam["fp16"]
	}
	weightsGB := float64(*m.EstimatedParamBillions) * bpp
	availableGB := float64(gpus) * 24.0 * 0.8
	if weightsGB > availableGB {
		return fmt.Errorf(
			"model (~%.0fGB of weights at %s) does not fit in %d GPU(s) (~%.0fGB usable); request more GPUs or stronger quantization",
			weightsGB, quantOrDefault(m.Quantization), gpus, availableGB)
	}
	return nil
}

func quantOrDefault(q string) string {
	if q == "" {
		return "fp16"
	}
	return q
}
