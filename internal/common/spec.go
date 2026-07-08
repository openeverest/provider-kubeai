// Package common defines shared constants used across the provider.
package common

const (
	// ProviderName is the canonical name of this provider.
	ProviderName = "provider-kubeai"

	// ComponentServer is the logical component name for the inference server.
	ComponentServer = "server"

	// ComponentTypeVllm is the component type the server component runs.
	ComponentTypeVllm = "vllm"
)
