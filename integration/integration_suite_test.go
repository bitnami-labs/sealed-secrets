// +build integration

package integration

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	// For client auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
var kubesealBin = flag.String("kubeseal-bin", "kubeseal", "path to kubeseal executable under test")
var controllerBin = flag.String("controller-bin", "controller", "path to controller executable under test")

func clusterConfigOrDie() *rest.Config {
	var config *rest.Config
	var err error

	if *kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		panic(err.Error())
	}

	return config
}

func createNsOrDie(c corev1.NamespacesGetter, ns string) string {
	result, err := c.Namespaces().Create(
		&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ns,
			},
		})
	if err != nil {
		panic(err.Error())
	}
	name := result.GetName()
	fmt.Fprintf(GinkgoWriter, "Created namespace %s\n", name)
	return name
}

func deleteNsOrDie(c corev1.NamespacesGetter, ns string) {
	err := c.Namespaces().Delete(ns, &metav1.DeleteOptions{})
	if err != nil {
		panic(err.Error())
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func runKubeseal(flags []string, input io.Reader, output io.Writer, opts ...runAppOpt) error {
	args := []string{}
	if *kubeconfig != "" && !containsString(flags, "--kubeconfig") {
		args = append(args, "--kubeconfig", *kubeconfig)
	}
	args = append(args, flags...)

	return runApp(*kubesealBin, args, input, output, opts...)
}

type interruptableReader struct {
	ctx context.Context
	r   io.Reader
}

func (r interruptableReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	if err != nil {
		return n, err
	}
	return n, r.ctx.Err()
}

func streamLog(ctx context.Context, c corev1.PodsGetter, namespace, name, container string, output io.Writer, prefix string) error {
	zero := int64(0)
	readCloser, err := c.Pods(namespace).GetLogs(name, &v1.PodLogOptions{
		Container:    container,
		Follow:       true,
		SinceSeconds: &zero,
	}).Stream()
	if err != nil {
		return err
	}
	defer readCloser.Close()

	scanner := bufio.NewScanner(interruptableReader{ctx, readCloser})
	for scanner.Scan() {
		fmt.Fprintf(output, "%s%s\n", prefix, scanner.Text())
	}
	return scanner.Err()
}

func runController(flags []string, input io.Reader, output io.Writer) error {
	return runApp(*controllerBin, flags, input, output)
}

type runAppOpt func(*runAppOpts)

type runAppOpts struct {
	stderr io.Writer
}

func runAppWithStderr(w io.Writer) runAppOpt {
	return func(o *runAppOpts) { o.stderr = w }
}

func runApp(app string, flags []string, input io.Reader, output io.Writer, opts ...runAppOpt) error {
	options := runAppOpts{
		stderr: GinkgoWriter,
	}
	for _, o := range opts {
		o(&options)
	}

	fmt.Fprintf(GinkgoWriter, "Running %q %q\n", app, flags)
	cmd := exec.Command(app, flags...)
	cmd.Stdin = input
	cmd.Stdout = output
	cmd.Stderr = options.stderr

	return cmd.Run()
}

func runKubesealWith(flags []string, input runtime.Object, opts ...runAppOpt) (runtime.Object, error) {
	enc := scheme.Codecs.LegacyCodec(v1.SchemeGroupVersion)
	indata, err := runtime.Encode(enc, input)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(GinkgoWriter, "kubeseal input:\n%s\n", indata)

	outbuf := bytes.Buffer{}

	if err := runKubeseal(flags, bytes.NewReader(indata), &outbuf, opts...); err != nil {
		return nil, err
	}

	fmt.Fprintf(GinkgoWriter, "kubeseal output:\n%s\n", outbuf.Bytes())

	outputObj, err := runtime.Decode(scheme.Codecs.UniversalDecoder(ssv1alpha1.SchemeGroupVersion), outbuf.Bytes())
	if err != nil {
		return nil, err
	}

	return outputObj, nil
}

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "sealed-secrets integration tests")
}
