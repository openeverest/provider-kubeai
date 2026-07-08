// Package autoscaled contains custom spec types for autoscales topology
// +k8s:openapi-gen=true
package autoscaled

// AutoScaledToplogyConfig maps to Instance.spec.topology.config for the autoscaled topology.
// It configures Kube-AI request based autoscaler capped between minReplicas to maxReplicas
type AutoScaledTopologyConfig struct {
	// MinReplicas is the lower bound of server pods. zero here as it enables scale-to-zero
	// +kubebuilder:validation:Minimum=0
	MinReplicas int32 `json:"minReplicas"`

	// MaxReplicas is the upper bound of server pods
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// TargetRequests is the avg no. of in flight requests per pod the autoscaler tries to maintain,
	// Default for KubeAI is 100
	// +k8sbuilder:validation:Minimum=1
	TargetRequests *int32 `json:"targetRequests,omitempty"`

	// ScaleDownDelaySeconds is the minimum time before scaling down after the
	// autoscaler decides to. Defaults to KubeAI's default (30).
	ScaleDownDelaySeconds *int64 `json:"scaleDownDelaySeconds,omitempty"`
}
