package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "github.com/openeverest/openeverest/v2/api/core/v1alpha1"
	"github.com/openeverest/openeverest/v2/provider-runtime/controller/validationtest"

	"github.com/openeverest/provider-kubeai/internal/common"
)

const (
	providerSpecPath  = "../../charts/provider-kubeai/generated/provider-spec.yaml"
	autoscaledFixture = "testdata/validate/autoscaled.yaml"
	gpuResource       = "nvidia.com/gpu"
	serverPath        = "spec.components[server]"
	sourcePath        = serverPath + ".parameters.model.source"
)

// server applies mutate to the server component of the Instance.
func server(mutate func(*corev1alpha1.ComponentSpec)) func(*corev1alpha1.Instance) {
	return func(in *corev1alpha1.Instance) {
		c := in.Spec.Components[common.ComponentServer]
		mutate(&c)
		in.Spec.Components[common.ComponentServer] = c
	}
}

// serverParameters sets the server component's parameters to raw JSON.
func serverParameters(raw string) func(*corev1alpha1.Instance) {
	return server(func(c *corev1alpha1.ComponentSpec) { c.Parameters = &runtime.RawExtension{Raw: []byte(raw)} })
}

// TestValidate runs the checks the runtime runs before every Sync: the
// schemas, supportedFields and catalog the provider publishes, then Validate.
func TestValidate(t *testing.T) {
	t.Parallel()
	v := validationtest.New(t, New(), providerSpecPath)

	tests := []struct {
		name   string
		mutate func(*corev1alpha1.Instance)
		want   []string
	}{
		{name: "valid autoscaled"},
		{
			name:   "no topology type",
			mutate: func(in *corev1alpha1.Instance) { in.Spec.Topology = nil },
		},
		{
			name: "ollama model on CPU",
			mutate: server(func(c *corev1alpha1.ComponentSpec) {
				c.Parameters = &runtime.RawExtension{Raw: []byte(`{"model":{"source":"ollama://qwen2:0.5b"}}`)}
				c.Resources = nil
			}),
		},
		{
			name: "large model that fits with int4 on two GPUs",
			mutate: func(in *corev1alpha1.Instance) {
				serverParameters(`{"model":{"source":"hf://meta-llama/Llama-3.1-70B-Instruct","estimatedParamBillions":70,"quantization":"int4"}}`)(in)
				server(func(c *corev1alpha1.ComponentSpec) { c.Resources.Limits[gpuResource] = resource.MustParse("2") })(in)
			},
		},
		{
			name:   "unknown version",
			mutate: func(in *corev1alpha1.Instance) { in.Spec.Version = "vllm-9.9" },
			want:   []string{"spec.version"},
		},
		{
			name: "task KubeAI has no feature for",
			mutate: func(in *corev1alpha1.Instance) {
				in.Spec.Parameters = &runtime.RawExtension{Raw: []byte(`{"task":"ImageGeneration"}`)}
			},
			want: []string{"spec.parameters.task"},
		},
		{
			name:   "server type other than vllm",
			mutate: server(func(c *corev1alpha1.ComponentSpec) { c.Type = "sglang" }),
			want:   []string{serverPath + ".type"},
		},
		{
			name:   "missing server",
			mutate: func(in *corev1alpha1.Instance) { delete(in.Spec.Components, common.ComponentServer) },
			want:   []string{serverPath},
		},
		{
			name: "missing server without a topology type",
			mutate: func(in *corev1alpha1.Instance) {
				in.Spec.Topology = nil
				delete(in.Spec.Components, common.ComponentServer)
			},
			want: []string{serverPath},
		},
		{
			name:   "server without parameters",
			mutate: server(func(c *corev1alpha1.ComponentSpec) { c.Parameters = nil }),
			want:   []string{serverPath + ".parameters"},
		},
		{
			name:   "model without a source",
			mutate: serverParameters(`{"model":{}}`),
			want:   []string{sourcePath},
		},
		{
			name:   "empty model source",
			mutate: serverParameters(`{"model":{"source":""}}`),
			want:   []string{sourcePath},
		},
		{
			name:   "model source with an unsupported scheme",
			mutate: serverParameters(`{"model":{"source":"https://example.invalid/model"}}`),
			want:   []string{sourcePath},
		},
		{
			name: "maxReplicas below minReplicas",
			mutate: func(in *corev1alpha1.Instance) {
				in.Spec.Topology.Parameters = &runtime.RawExtension{Raw: []byte(`{"minReplicas":3,"maxReplicas":2}`)}
			},
			want: []string{"spec.topology.parameters.maxReplicas"},
		},
		{
			name:   "model larger than its GPUs",
			mutate: serverParameters(`{"model":{"source":"hf://meta-llama/Llama-3.1-70B-Instruct","estimatedParamBillions":70}}`),
			want:   []string{serverPath + ".resources.limits[" + gpuResource + "]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := validationtest.Load(t, autoscaledFixture)
			if tt.mutate != nil {
				tt.mutate(in)
			}

			res := v.Run(in, nil)

			var got []string
			for _, err := range res.Errors {
				got = append(got, err.Field)
			}
			assert.Equal(t, tt.want, got, "errors: %v", res.Errors)
		})
	}
}
