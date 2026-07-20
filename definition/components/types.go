// Package components contains parameters types for provider component types.
//
// Each struct here corresponds to a component type defined in versions.yaml
// and is converted to an OpenAPI schema during generation.
// Add fields when a component type needs custom configuration beyond
// what the base Instance spec provides.
//
// +k8s:openapi-gen=true
package components

// VllmParameters maps to Instance.spec.components.server.parameters.
// It carries the LLM-specific configuration that has no equivalent in the
// generic Instance component spec of OpenEverest (replicas/resources/storage/version).

type VllmParameters struct {
	// Model describes the model artifact to serve
	Model ModelSpec `json:"model"`

	// Args are extra engine flags appended
	// e.g. --max-model-len=8192
	Args []string `json:"args,omitempty"`

	// Env variables to be added to the server process
	Env map[string]string `json:"env,omitempty"`

	// ResourceProfile is the KubeAI resource profile in the form
	// "<profile-name>:<count>", e.g. "cpu:2" or "nvidia-gpu-l4:1"
	ResourceProfile string `json:"resourceProfile,omitempty"`

	// CacheProfile enables model artifact caching using a KubeAI CacheProfile
	CacheProfile string `json:"cacheProfile,omitempty"`
}

// ModelSpec describes where the model weights actually come from and hints for placement validation
// for e.g; vllm: "hf://<repo_name>/<model>", "pvc://<name>", or "s3://..."
// we can also serve ollama for dev testing of entire provider flow which can run on CPU
// without need of GPUs e.g; Ollama: "ollama://<model>"
type ModelSpec struct {
	// Source is the model URL understood by the serving engine.
	Source string `json:"source"`

	// EstimatedParamBillions is an optional hint (model size in billions of params)
	// used by Validate() for a GPU memory
	EstimatedParamBillions *int32 `json:"estimatedParamBillions,omitempty"`

	// Quantization declares the weight precision for the fit heuristic.
	// +kubebuilder:validation:Enum=fp16;int8;int4
	Quantization string `json:"quantization,omitempty"`
}
