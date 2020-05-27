package utils

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MyNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return metav1.NamespaceDefault
}

// ScheduleJobWithTrigger creates a long-running loop that runs a job after an initialDelay
// and then after each period duration.
// It returns a trigger function that runs the job early when called.
func ScheduleJobWithTrigger(initialDelay, period time.Duration, job func()) func() {
	trigger := make(chan struct{})
	go func() {
		for {
			<-trigger
			job()
		}
	}()
	go func() {
		time.Sleep(initialDelay)
		for {
			trigger <- struct{}{}
			time.Sleep(period)
		}
	}()
	return func() {
		trigger <- struct{}{}
	}
}