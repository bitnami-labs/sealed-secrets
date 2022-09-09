package controller

import (
	"fmt"
	"strings"
	"time"
)

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

const (
	kubeChars     = "abcdefghijklmnopqrstuvwxyz0123456789-" // Acceptable characters in k8s resource name
	maxNameLength = 245                                     // Max resource name length is 253, leave some room for a suffix
)

func validateKeyPrefix(name string) (string, error) {
	if len(name) > maxNameLength {
		return "", fmt.Errorf("name is too long, must be shorter than %d, got %d", maxNameLength, len(name))
	}
	for _, char := range name {
		if !strings.ContainsRune(kubeChars, char) {
			return "", fmt.Errorf("name contains illegal character %c", char)
		}
	}
	return name, nil
}

func removeDuplicates(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}
