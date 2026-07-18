// Package definition contains shared types used across the provider definitions

// +k8s:openapi-gen=true
package definition

// TopologyType enumerates the deployment technologies the provider supports
type TopologyType string

const (
	// TopologyAutoscaled deploys an inference server that scales between
	// minReplicas and maxReplicas based on request load (KubeAI autoscaler)
	TopologyAutoscaled TopologyType = "autoscaled"
)

// GlobalConfig maps to Instance.spec.global.
type GlobalConfig struct {
	// Task selects the model capability, which dictates openAI compatible APIs exposed for the model
	// +kubebuilder:validation:Enum=TextGeneration;TextEmbedding;SpeechToText
	Task string `json:"task,omitempty"`
}
