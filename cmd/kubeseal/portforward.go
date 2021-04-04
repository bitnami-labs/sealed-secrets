package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

var (
	errServiceNotFound = errors.New("could not find service for port forwarding")
	errPodNotFound     = errors.New("could not find a matching pod for port forwarding")
	errPodPortNotFound = errors.New("could not find a valid container port for port forwarding")
)

func openCertFromPortForward(namespace, name string, port int32) (io.ReadCloser, error) {
	// Setup client configs.
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return nil, err
	}

	// Grab the service controller.
	svc, err := getControllerService(restClient, name, namespace)
	if err != nil {
		return nil, err
	}

	// Grab a pod based on the spec from the service.
	pod, err := getControllerPod(restClient, svc, namespace)
	if err != nil {
		return nil, err
	}

	// Find the port we want to forward from within the pods container(s).
	podPort, err := getControllerPodPort(pod)
	if err != nil {
		return nil, err
	}

	// Setup the port forwarding path.
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, pod.Name)

	// Setup transport and dialer for port forward api call.
	transport, upgrader, err := spdy.RoundTripperFor(conf)
	if err != nil {
		return nil, err
	}
	dialer := spdy.NewDialer(
		upgrader,
		&http.Client{Transport: transport},
		http.MethodPost,
		&url.URL{Scheme: "https", Path: path, Host: strings.TrimLeft(conf.Host, "htps:/")},
	)

	// Channels used for notifying status of port forward call.
	stopCh := make(chan struct{}, 1)
	readyCh := make(chan struct{})
	errorCh := make(chan error)

	// If we run into any issues, always send the stop signal.
	defer func() { stopCh <- struct{}{} }()

	// Create a new PortForwarder, keep the output clean and discard stdout/stderr.
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", port, podPort)}, stopCh, readyCh, io.Discard, io.Discard)
	if err != nil {
		return nil, err
	}

	// Start the PortForwarder in a goroutine, capture the error if possible.
	go func() { errorCh <- fw.ForwardPorts() }()

	// Wait for the port to be ready, error out, or time out.
	select {
	case <-readyCh:
		return fetchControllerCertOverPort(port)
	case err := <-errorCh:
		return nil, err
	case <-time.After(15 * time.Second):
		return nil, errors.New("port forward timed out")
	}
}

func getControllerService(client corev1.CoreV1Interface, name, namespace string) (*v1.Service, error) {
	// Grab the service controller.
	services, err := client.Services(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Iterate over the services until we find our match.
	for _, svc := range services.Items {
		if strings.EqualFold(svc.Name, name) {
			return &svc, nil
		}
	}

	return nil, errServiceNotFound
}

func getControllerPod(client corev1.CoreV1Interface, svc *v1.Service, namespace string) (*v1.Pod, error) {
	// Find a pod from the service details.
	listOptions := metav1.ListOptions{LabelSelector: labels.Set(svc.Spec.Selector).AsSelector().String()}
	pods, err := client.Pods(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		return &pod, nil
	}

	return nil, errPodNotFound
}

func getControllerPodPort(pod *v1.Pod) (int32, error) {
	// Find the port we want to forward from the pod.
	var podPort int32
	for _, container := range pod.Spec.Containers {
		for _, containerPort := range container.Ports {
			// We should only have one port, but if there are more try to grab the `http` port.
			if podPort == 0 || strings.Contains(strings.ToLower(containerPort.Name), "http") {
				podPort = containerPort.ContainerPort
			}
		}
	}

	// If we found a valid pod port, return it.
	if podPort > 0 {
		return podPort, nil
	}

	return 0, errPodPortNotFound
}

func fetchControllerCertOverPort(port int32) (io.ReadCloser, error) {
	// Port is ready, fetch the cert with a timeout.
	client := &http.Client{Timeout: 15 * time.Second}

	// Create the request from the given port.
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d%s", port, certRequestPath), nil)
	if err != nil {
		return nil, err
	}

	// Add our accept header, make the request.
	req.Header.Set("Accept", certAcceptContentTypes)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Return the body stream.
	return resp.Body, nil
}
