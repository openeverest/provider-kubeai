package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubeaiv1 "github.com/kubeai-project/kubeai/api/k8s/v1"
	corev1alpha1 "github.com/openeverest/openeverest/v2/api/core/v1alpha1"
	"github.com/openeverest/openeverest/v2/provider-runtime/controller"

	"github.com/openeverest/provider-kubeai/internal/common"
)

func TestStatus(t *testing.T) {
	t.Parallel()

	in := &corev1alpha1.Instance{ObjectMeta: metav1.ObjectMeta{Name: "qwen", Namespace: "ai"}}
	model := func(minReplicas, all, ready int32, cache *kubeaiv1.ModelStatusCache) *kubeaiv1.Model {
		return &kubeaiv1.Model{
			ObjectMeta: metav1.ObjectMeta{Name: in.Name, Namespace: in.Namespace},
			Spec:       kubeaiv1.ModelSpec{MinReplicas: minReplicas},
			Status: kubeaiv1.ModelStatus{
				Replicas: kubeaiv1.ModelStatusReplicas{All: all, Ready: ready},
				Cache:    cache,
			},
		}
	}
	details := controller.ConnectionDetails{
		Type:     "llm",
		Provider: common.ProviderName,
		Host:     "kubeai.ai.svc.cluster.local",
		Port:     "80",
		URI:      "http://kubeai.ai.svc.cluster.local/openai/v1",
		AdditionalProperties: map[string]string{
			"modelName": "qwen",
			"basePath":  "/openai/v1",
		},
	}

	tests := []struct {
		name  string
		model *kubeaiv1.Model
		want  controller.Status
	}{
		{
			name: "model not created yet",
			want: controller.Progressing("waiting for KubeAI model to be created"),
		},
		{
			name:  "replica ready",
			model: model(1, 1, 1, nil),
			want:  controller.Status{Converged: true, ConnectionDetails: details},
		},
		{
			name:  "scaled up from zero and serving",
			model: model(0, 2, 1, nil),
			want:  controller.Status{Converged: true, ConnectionDetails: details},
		},
		{
			name:  "loading weights into the cache",
			model: model(0, 0, 0, &kubeaiv1.ModelStatusCache{Loaded: false}),
			want:  controller.Progressing("downloading model weights into cache"),
		},
		{
			name:  "scaled to zero",
			model: model(0, 0, 0, nil),
			want: controller.Status{
				Converged:         true,
				Available:         controller.Available,
				AvailableReason:   "Idle",
				AvailableMessage:  "Scaled to zero: the next request starts the model",
				ConnectionDetails: details,
			},
		},
		{
			name:  "scaled to zero with the cache loaded",
			model: model(0, 0, 0, &kubeaiv1.ModelStatusCache{Loaded: true}),
			want: controller.Status{
				Converged:         true,
				Available:         controller.Available,
				AvailableReason:   "Idle",
				AvailableMessage:  "Scaled to zero: the next request starts the model",
				ConnectionDetails: details,
			},
		},
		{
			name:  "starting the first replica",
			model: model(1, 0, 0, nil),
			want:  controller.Progressing("waiting for model server pods to become ready"),
		},
		{
			name:  "scaling up from zero",
			model: model(0, 1, 0, nil),
			want:  controller.Progressing("waiting for model server pods to become ready"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme := runtime.NewScheme()
			require.NoError(t, kubeaiv1.SchemeBuilder.AddToScheme(scheme))
			b := fake.NewClientBuilder().WithScheme(scheme)
			if tt.model != nil {
				b = b.WithObjects(tt.model)
			}
			c := controller.NewContext(t.Context(), b.Build(), in, common.ProviderName)

			got, err := New().Status(c)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
