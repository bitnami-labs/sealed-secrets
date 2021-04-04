package main

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetMissingControllerService(t *testing.T) {
	fake.NewSimpleClientset()
	client := fake.NewSimpleClientset().CoreV1()
	name := "controller-name"
	ns := "kube-system"

	svc, err := getControllerService(client, name, ns)
	if svc != nil {
		t.Fatal("service should be nil")
	}
	if err != errServiceNotFound {
		t.Fatal("error should be errServiceNotFound")
	}
}

func TestGetCorrectControllerService(t *testing.T) {
	fake.NewSimpleClientset()
	client := fake.NewSimpleClientset().CoreV1()
	name := "controller-name"
	ns := "kube-system"

	// Add a few services.
	_, err1 := client.Services(ns).Create(&v1.Service{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo1": "bar1"},
		},
		Status: v1.ServiceStatus{},
	})
	if err1 != nil {
		t.Fatal("creating first service failed")
	}

	_, err2 := client.Services(ns).Create(&v1.Service{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: name + "2"},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo2": "bar2"},
		},
		Status: v1.ServiceStatus{},
	})
	if err2 != nil {
		t.Fatal("creating second service failed")
	}

	svc, err := getControllerService(client, name, ns)
	if svc == nil {
		t.Fatal("service should not be nil")
	}
	if err != nil {
		t.Fatal("service should not have error")
	}
	if svc.Name != name {
		t.Fatal("service names did not match")
	}
	if svc.Spec.Selector["foo1"] != "bar1" {
		t.Fatal("service selectors did not match")
	}
}

func TestGetMissingControllerPod(t *testing.T) {
	fake.NewSimpleClientset()
	client := fake.NewSimpleClientset().CoreV1()
	name := "controller-name"
	ns := "kube-system"

	// Create an empty service.
	svc, err := client.Services(ns).Create(&v1.Service{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo1": "bar1"},
		},
		Status: v1.ServiceStatus{},
	})
	if err != nil {
		t.Fatal("creating service failed")
	}

	pod, err := getControllerPod(client, svc, ns)
	if pod != nil {
		t.Fatal("pod should be nil")
	}
	if err != errPodNotFound {
		t.Fatal("error should be errPodNotFound")
	}
}

func TestGetControllerPod(t *testing.T) {
	fake.NewSimpleClientset()
	client := fake.NewSimpleClientset().CoreV1()
	name := "controller-name"
	ns := "kube-system"
	goodLabels := map[string]string{"foo": "bar", "status": "good"}
	badLabels := map[string]string{"foo": "bar", "status": "bad"}

	// Create a good pod.
	_, err := client.Pods(ns).Create(&v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name + "1",
			Labels: goodLabels,
		},
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{},
	})
	if err != nil {
		t.Fatal("creating good pod failed")
	}

	// Create a bad pod.
	_, err = client.Pods(ns).Create(&v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name + "2",
			Labels: badLabels,
		},
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{},
	})
	if err != nil {
		t.Fatal("creating bad pod failed")
	}

	// Create a good service.
	svc, err := client.Services(ns).Create(&v1.Service{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.ServiceSpec{
			Selector: goodLabels,
		},
		Status: v1.ServiceStatus{},
	})
	if err != nil {
		t.Fatal("creating second service failed")
	}

	// Run test.
	pod, err := getControllerPod(client, svc, ns)
	if pod == nil {
		t.Fatal("pod should not be nil")
	}
	if err != nil {
		t.Fatal("should not error")
	}

	if !reflect.DeepEqual(pod.Labels, goodLabels) {
		t.Fatal("wrong pod selected")
	}
}

func TestGetMissingControllerPodPort(t *testing.T) {
	fake.NewSimpleClientset()
	client := fake.NewSimpleClientset().CoreV1()
	name := "controller-name"
	ns := "kube-system"
	labels := map[string]string{"foo": "bar", "status": "good"}

	// Create an empty service.
	pod, err := client.Pods(ns).Create(&v1.Pod{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec:       v1.PodSpec{Containers: []v1.Container{}},
		Status:     v1.PodStatus{},
	})
	if pod == nil {
		t.Fatal("pod should not be nil")
	}
	if err != nil {
		t.Fatal("error should be nil")
	}

	_, err = getControllerPodPort(pod)
	if err != errPodPortNotFound {
		t.Fatal("error should be errPodPortNotFound")
	}
}

func TestGetControllerPodPortSingle(t *testing.T) {
	fake.NewSimpleClientset()
	client := fake.NewSimpleClientset().CoreV1()
	name := "controller-name"
	ns := "kube-system"
	port := int32(12345)

	// Create an empty service.
	pod, err := client.Pods(ns).Create(&v1.Pod{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.PodSpec{Containers: []v1.Container{
			{
				Name: "foobar",
				Ports: []v1.ContainerPort{{
					Name:          "foo",
					ContainerPort: port,
				}},
			},
		}},
		Status: v1.PodStatus{},
	})
	if err != nil {
		t.Fatal("error should be nil")
	}
	if pod == nil {
		t.Fatal("pod should not be nil")
	}

	result, err := getControllerPodPort(pod)
	if err != nil {
		t.Fatal("error should be nil")
	}
	if result != port {
		t.Fatal("bad port returned")
	}
}

func TestGetControllerPodPortMulti(t *testing.T) {
	fake.NewSimpleClientset()
	client := fake.NewSimpleClientset().CoreV1()
	name := "controller-name"
	ns := "kube-system"
	port := int32(12345)

	// Create an empty service.
	pod, err := client.Pods(ns).Create(&v1.Pod{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.PodSpec{Containers: []v1.Container{
			{
				Name: "foobar",
				Ports: []v1.ContainerPort{
					{
						Name:          "abc",
						ContainerPort: 1025,
					},
					{
						Name:          "yxz",
						ContainerPort: 8080,
					},
					{
						Name:          "http",
						ContainerPort: port,
					},
				},
			},
		}},
		Status: v1.PodStatus{},
	})
	if err != nil {
		t.Fatal("error should be nil")
	}
	if pod == nil {
		t.Fatal("pod should not be nil")
	}

	result, err := getControllerPodPort(pod)
	if err != nil {
		t.Fatal("error should be nil")
	}
	if result != port {
		t.Fatal("bad port returned")
	}
}
