package provider

// Run `make manifests` to regenerate config/rbac/role.yaml from these markers.
// This file contains kubebuilder RBAC markers for controller-gen.
// See: https://book.kubebuilder.io/reference/markers/rbac

// Base RBAC (required by all providers):
// +kubebuilder:rbac:groups=core.openeverest.io,resources=instances,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core.openeverest.io,resources=instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.openeverest.io,resources=instances/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.openeverest.io,resources=providers,verbs=get;list;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// KubeAI Model CRs (created and owned by Sync):
// +kubebuilder:rbac:groups=kubeai.org,resources=models,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeai.org,resources=models/status,verbs=get
// +kubebuilder:rbac:groups=kubeai.org,resources=models/finalizers,verbs=update

// Connection details Secret written by the runtime:
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
