package controller

import (
	"testing"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// setupTestMetrics creates a fresh metrics setup for testing
func setupTestMetrics() *prometheus.Registry {
	registry := prometheus.NewRegistry()

	// Create a new conditionInfo metric for testing
	testConditionInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Name:      "condition_info",
			Help:      "Current SealedSecret condition status. Values are -1 (false), 0 (unknown or absent), 1 (true)",
		},
		[]string{labelNamespace, labelName, labelCondition, labelInstance},
	)

	registry.MustRegister(testConditionInfo)

	// Replace the global conditionInfo for testing
	conditionInfo = testConditionInfo

	return registry
}

func TestObserveCondition(t *testing.T) {
	registry := setupTestMetrics()

	ssecret := &ssv1alpha1.SealedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-secret",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "test-instance",
			},
		},
		Status: &ssv1alpha1.SealedSecretStatus{
			Conditions: []ssv1alpha1.SealedSecretCondition{
				{
					Type:   ssv1alpha1.SealedSecretSynced,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	ObserveCondition(ssecret)

	// Verify metric was created
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "sealed_secrets_controller_condition_info" {
			for _, metric := range mf.GetMetric() {
				labels := metric.GetLabel()
				if getLabel(labels, "namespace") == "test-ns" &&
					getLabel(labels, "name") == "test-secret" &&
					getLabel(labels, "condition") == "Synced" &&
					getLabel(labels, "ss_app_kubernetes_io_instance") == "test-instance" {
					found = true
					if metric.GetGauge().GetValue() != 1.0 {
						t.Errorf("Expected metric value 1.0, got %f", metric.GetGauge().GetValue())
					}
				}
			}
		}
	}

	if !found {
		t.Error("Expected metric not found")
	}
}

func TestUnregisterCondition(t *testing.T) {
	registry := setupTestMetrics()

	ssecret := &ssv1alpha1.SealedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-secret",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "test-instance",
			},
		},
		Status: &ssv1alpha1.SealedSecretStatus{
			Conditions: []ssv1alpha1.SealedSecretCondition{
				{
					Type:   ssv1alpha1.SealedSecretSynced,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	// First observe the condition to create the metric
	ObserveCondition(ssecret)

	// Verify metric exists
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	metricExists := func() bool {
		for _, mf := range metricFamilies {
			if mf.GetName() == "sealed_secrets_controller_condition_info" {
				for _, metric := range mf.GetMetric() {
					labels := metric.GetLabel()
					if getLabel(labels, "namespace") == "test-ns" &&
						getLabel(labels, "name") == "test-secret" &&
						getLabel(labels, "condition") == "Synced" &&
						getLabel(labels, "ss_app_kubernetes_io_instance") == "test-instance" {
						return true
					}
				}
			}
		}
		return false
	}

	if !metricExists() {
		t.Fatal("Metric should exist before unregistering")
	}

	// Now unregister the condition
	UnregisterCondition(ssecret)

	// Verify metric was removed
	metricFamilies, err = registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if metricExists() {
		t.Error("Metric should have been removed after unregistering")
	}
}

func TestUnregisterConditionWithNilStatus(t *testing.T) {
	ssecret := &ssv1alpha1.SealedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-secret",
		},
		Status: nil,
	}

	// Should not panic
	UnregisterCondition(ssecret)
}

func TestObserveConditionWithNilStatus(t *testing.T) {
	ssecret := &ssv1alpha1.SealedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-secret",
		},
		Status: nil,
	}

	// Should not panic
	ObserveCondition(ssecret)
}

func TestUnregisterConditionWithMissingLabel(t *testing.T) {
	registry := setupTestMetrics()

	ssecret := &ssv1alpha1.SealedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-secret",
			// Missing app.kubernetes.io/instance label
		},
		Status: &ssv1alpha1.SealedSecretStatus{
			Conditions: []ssv1alpha1.SealedSecretCondition{
				{
					Type:   ssv1alpha1.SealedSecretSynced,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	// First observe the condition to create the metric (with empty instance label)
	ObserveCondition(ssecret)

	// Now unregister the condition - should work with empty instance label
	UnregisterCondition(ssecret)

	// Verify metric was removed
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	for _, mf := range metricFamilies {
		if mf.GetName() == "sealed_secrets_controller_condition_info" {
			for _, metric := range mf.GetMetric() {
				labels := metric.GetLabel()
				if getLabel(labels, "namespace") == "test-ns" &&
					getLabel(labels, "name") == "test-secret" &&
					getLabel(labels, "condition") == "Synced" &&
					getLabel(labels, "ss_app_kubernetes_io_instance") == "" {
					t.Error("Metric should have been removed after unregistering")
				}
			}
		}
	}
}

// Helper function to get label value from metric labels
func getLabel(labels []*dto.LabelPair, name string) string {
	for _, label := range labels {
		if label.GetName() == name {
			return label.GetValue()
		}
	}
	return ""
}
