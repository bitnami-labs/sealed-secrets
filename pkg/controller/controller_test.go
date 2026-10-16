package controller

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"encoding/json"

	ssv1alpha1 "github.com/bitnami/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	ssfake "github.com/bitnami/sealed-secrets/pkg/client/clientset/versioned/fake"
	"github.com/bitnami/sealed-secrets/pkg/crypto"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func someStr(s string) *string {
	return &s
}

func TestIsAnnotatedToBePatched(t *testing.T) {
	tests := []struct {
		annotations map[string]string
		want        bool
	}{
		{annotations: map[string]string{ssv1alpha1.SealedSecretPatchAnnotation: "true"}, want: true},
		{annotations: map[string]string{ssv1alpha1.SealedSecretPatchAnnotation: "TRUE"}, want: false},
		{annotations: map[string]string{ssv1alpha1.SealedSecretPatchAnnotation: "false"}, want: false},
		{annotations: map[string]string{ssv1alpha1.SealedSecretPatchAnnotation: ""}, want: false},
		{annotations: map[string]string{"something": "else"}, want: false},
		{annotations: map[string]string{}, want: false},
	}

	for i, tc := range tests {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   "test-ns",
				Name:        "test-secret",
				Annotations: tc.annotations,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		got := isAnnotatedToBePatched(s)
		if got != tc.want {
			t.Fatalf("test %d: expected: %v, got: %v", i+1, tc.want, got)
		}
	}
}

func TestIsAnnotatedToBeManaged(t *testing.T) {
	tests := []struct {
		annotations map[string]string
		want        bool
	}{
		{annotations: map[string]string{ssv1alpha1.SealedSecretManagedAnnotation: "true"}, want: true},
		{annotations: map[string]string{ssv1alpha1.SealedSecretManagedAnnotation: "TRUE"}, want: false},
		{annotations: map[string]string{ssv1alpha1.SealedSecretManagedAnnotation: "false"}, want: false},
		{annotations: map[string]string{ssv1alpha1.SealedSecretManagedAnnotation: ""}, want: false},
		{annotations: map[string]string{"something": "else"}, want: false},
		{annotations: map[string]string{}, want: false},
	}

	for i, tc := range tests {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   "test-ns",
				Name:        "test-secret",
				Annotations: tc.annotations,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		got := isAnnotatedToBeManaged(s)
		if got != tc.want {
			t.Fatalf("test %d: expected: %v, got: %v", i+1, tc.want, got)
		}
	}
}

func TestConvert2SealedSecretBadType(t *testing.T) {
	obj := struct{}{}
	_, got := convertSealedSecret(obj)
	want := ErrCast
	if !errors.Is(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestConvert2SealedSecretFills(t *testing.T) {
	sealedSecret := ssv1alpha1.SealedSecret{}

	result, err := convertSealedSecret(any(&sealedSecret))
	if err != nil {
		t.Fatalf("unexpected failure converting to a sealed secret: %v", err)
	}
	got := fmt.Sprintf("%s %s", result.APIVersion, result.Kind)
	want := "bitnami.com/v1alpha1 SealedSecret"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestConvert2SealedSecretPassThrough(t *testing.T) {
	sealedSecret := ssv1alpha1.SealedSecret{}
	sealedSecret.APIVersion = "bitnami.com/v1alpha1"
	sealedSecret.Kind = "SealedSecrets"

	want := &sealedSecret
	got, err := convertSealedSecret(any(want))
	if err != nil {
		t.Fatalf("unexpected failure converting to a sealed secret: %v", err)
	}
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDefaultConfigDoesNotSkipRecreate(t *testing.T) {
	ns := "some-namespace"
	keyNs := "some-key-namespace"
	var tweakopts func(*metav1.ListOptions)
	clientset := fake.NewClientset()
	ssc := ssfake.NewSimpleClientset()
	keyRegistry := testKeyRegister(t, context.Background(), clientset, ns)

	got, err := prepareController(clientset, ns, keyNs, tweakopts, &Flags{SkipRecreate: false}, ssc, keyRegistry)
	if err != nil {
		t.Fatalf("err %v want %v", got, nil)
	}
	if got == nil {
		t.Fatalf("ctrl %v want non nil", got)
	}
	if got.sInformer == nil {
		t.Fatalf("sInformer %v want non nil", got.sInformer)
	}
}

func TestSkipRecreateConfigDoesSkipIt(t *testing.T) {
	ns := "some-namespace"
	keyNs := "some-key-namespace"
	var tweakopts func(*metav1.ListOptions)
	clientset := fake.NewClientset()
	ssc := ssfake.NewSimpleClientset()
	keyRegistry := testKeyRegister(t, context.Background(), clientset, ns)

	got, err := prepareController(clientset, ns, keyNs, tweakopts, &Flags{SkipRecreate: true}, ssc, keyRegistry)
	if err != nil {
		t.Fatalf("err %v want %v", got, nil)
	}
	if got == nil {
		t.Fatalf("ctrl %v want non nil", got)
	}
	if got.sInformer != nil {
		t.Fatalf("sInformer %v want nil", got.sInformer)
	}
}

func TestEmptyStatusSendsUpdate(t *testing.T) {
	updateRequired := updateSealedSecretsStatusConditions(&ssv1alpha1.SealedSecretStatus{}, nil)

	if !updateRequired {
		t.Fatalf("expected status update, but no update was send")
	}
}

func TestStatusUpdateSendsUpdate(t *testing.T) {
	status := &ssv1alpha1.SealedSecretStatus{
		Conditions: []ssv1alpha1.SealedSecretCondition{{
			Status:         "False",
			Type:           ssv1alpha1.SealedSecretSynced,
			LastUpdateTime: metav1.Now(),
		}},
	}
	updateRequired := updateSealedSecretsStatusConditions(status, nil)

	if !updateRequired {
		t.Fatalf("expected status update, but no update was send")
	}

	if status.Conditions[0].LastTransitionTime.IsZero() {
		t.Fatalf("expected LastTransitionTime is not empty")
	}

	if status.Conditions[0].LastUpdateTime.IsZero() {
		t.Fatalf("expected LastUpdateTime is not empty")
	}
}

func TestSameStatusNoUpdate(t *testing.T) {
	updateRequired := updateSealedSecretsStatusConditions(&ssv1alpha1.SealedSecretStatus{
		Conditions: []ssv1alpha1.SealedSecretCondition{{
			Type:   ssv1alpha1.SealedSecretSynced,
			Status: "False",
		}},
	}, errors.New("testerror"))

	if updateRequired {
		t.Fatalf("expected no status update, but update was send")
	}
}

func TestSyncedSecretWithErrorSendsUpdate(t *testing.T) {
	updateRequired := updateSealedSecretsStatusConditions(&ssv1alpha1.SealedSecretStatus{
		Conditions: []ssv1alpha1.SealedSecretCondition{{
			Type:   ssv1alpha1.SealedSecretSynced,
			Status: "True",
		}},
	}, errors.New("testerror"))

	if !updateRequired {
		t.Fatalf("expected status update, but no update was send")
	}
}

func testKeyRegister(t *testing.T, ctx context.Context, clientset kubernetes.Interface, ns string) *KeyRegistry {
	t.Helper()

	keyLabel := SealedSecretsKeyLabel
	prefix := "test-keys"
	testKeySize := 4096
	keyRegistry, err := initKeyRegistry(ctx, clientset, rand.Reader, ns, prefix, keyLabel, testKeySize, "CertNotBefore")
	if err != nil {
		t.Fatalf("failed to provision key registry: %v", err)
	}
	return keyRegistry
}

func prettyEncoder(codecs runtimeserializer.CodecFactory, mediaType string, gv runtime.GroupVersioner) (runtime.Encoder, error) {
	info, ok := runtime.SerializerInfoForMediaType(codecs.SupportedMediaTypes(), mediaType)
	if !ok {
		return nil, fmt.Errorf("binary can't serialize %s", mediaType)
	}

	prettyEncoder := info.PrettySerializer
	if prettyEncoder == nil {
		prettyEncoder = info.Serializer
	}

	enc := codecs.EncoderForVersion(prettyEncoder, gv)
	return enc, nil
}

func TestRotate(t *testing.T) {
	ns := "some-namespace"
	keyNs := "some-key-namespace"
	var tweakopts func(*metav1.ListOptions)
	clientset := fake.NewClientset()
	ssc := ssfake.NewSimpleClientset()
	keyRegistry := testKeyRegister(t, context.Background(), clientset, ns)

	// Add a key to the controller for second test
	validFor := time.Hour
	cn := "my-cn"
	_, err := keyRegistry.generateKey(context.Background(), validFor, cn, "", "")
	if err != nil {
		t.Fatal(err)
	}

	controller, err := prepareController(clientset, ns, keyNs, tweakopts, &Flags{SkipRecreate: false}, ssc, keyRegistry)
	if err != nil {
		t.Fatalf("err %v want %v", err, nil)
	}
	if controller == nil {
		t.Fatalf("ctrl %v want non nil", controller)
	}
	if controller.sInformer == nil {
		t.Fatalf("sInformer %v want non nil", controller.sInformer)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ss",
			Namespace: "default",
		},
		Data: map[string][]byte{
			// dGVtcG9yYWw= is base64 for "temporal"
			"password": []byte("temporal"),
		},
	}

	cert, err := controller.keyRegistry.getCert()
	if err != nil {
		t.Fatalf("error getting certificate: %v", err)
	}

	ssecret, err := ssv1alpha1.NewSealedSecret(scheme.Codecs, cert.PublicKey.(*rsa.PublicKey), secret)
	if err != nil {
		t.Fatalf("error creating sealed secrets: %v", err)
	}

	prettyEnc, err := prettyEncoder(scheme.Codecs, runtime.ContentTypeYAML, ssv1alpha1.SchemeGroupVersion)
	if err != nil {
		t.Fatalf("unexpected pretty encoding: %v", err)
	}

	data, err := runtime.Encode(prettyEnc, ssecret)
	if err != nil {
		t.Fatalf("unexpected encoding the sealed secret: %v", err)
	}

	got, err := controller.Rotate(data)
	if err != nil {
		t.Fatalf("unexpected failure converting to a sealed secret: %v", err)
	}
	if string(got) == string(data) {
		t.Fatalf("got %v want %v", string(got), string(data))
	}
}

func TestRotateKeepScope(t *testing.T) {
	ns := "some-namespace"
	keyNs := "some-key-namespace"
	var tweakopts func(*metav1.ListOptions)
	clientset := fake.NewClientset()
	ssc := ssfake.NewSimpleClientset()
	keyRegistry := testKeyRegister(t, context.Background(), clientset, ns)

	// Add a key to the controller for second test
	validFor := time.Hour
	cn := "my-cn"
	_, err := keyRegistry.generateKey(context.Background(), validFor, cn, "", "")
	if err != nil {
		t.Fatal(err)
	}

	controller, err := prepareController(clientset, ns, keyNs, tweakopts, &Flags{SkipRecreate: false}, ssc, keyRegistry)
	if err != nil {
		t.Fatalf("err %v want %v", err, nil)
	}
	if controller == nil {
		t.Fatalf("ctrl %v want non nil", controller)
	}
	if controller.sInformer == nil {
		t.Fatalf("sInformer %v want non nil", controller.sInformer)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ss",
			Namespace: "default",
		},
		Data: map[string][]byte{
			// dGVtcG9yYWw= is base64 for "temporal"
			"password": []byte("temporal"),
		},
	}

	cert, err := controller.keyRegistry.getCert()
	if err != nil {
		t.Fatalf("error getting certificate: %v", err)
	}

	ssecret, err := ssv1alpha1.NewSealedSecret(scheme.Codecs, cert.PublicKey.(*rsa.PublicKey), secret)
	if err != nil {
		t.Fatalf("error creating sealed secrets: %v", err)
	}
	ssecret.Spec.Template.ObjectMeta.Annotations = map[string]string{ssv1alpha1.SealedSecretClusterWideAnnotation: "true"}

	prettyEnc, err := prettyEncoder(scheme.Codecs, runtime.ContentTypeJSON, ssv1alpha1.SchemeGroupVersion)
	if err != nil {
		t.Fatalf("unexpected pretty encoding: %v", err)
	}

	data, err := runtime.Encode(prettyEnc, ssecret)
	if err != nil {
		t.Fatalf("unexpected encoding the sealed secret: %v", err)
	}

	out, err := controller.Rotate(data)
	if err != nil {
		t.Fatalf("expected failure is not hit")
	}

	s := &ssv1alpha1.SealedSecret{}
	if err = json.Unmarshal(out, s); err != nil {
		t.Fatalf("error unmarshalling the rotate sealed secret")
	}

	if ssv1alpha1.SecretScope(s) != ssv1alpha1.SecretScope(ssecret) {
		t.Fatalf("Scope from the original and the rotate sealed secret do not match")
	}
}

// TestAttemptUnsealIgnoresTemplate is a regression test for the /v1/verify decryption oracle.
func TestAttemptUnsealIgnoresTemplate(t *testing.T) {
	ns := "some-namespace"
	keyNs := "some-key-namespace"
	var tweakopts func(*metav1.ListOptions)
	clientset := fake.NewClientset()
	ssc := ssfake.NewSimpleClientset()
	keyRegistry := testKeyRegister(t, context.Background(), clientset, ns)

	validFor := time.Hour
	cn := "my-cn"
	_, err := keyRegistry.generateKey(context.Background(), validFor, cn, "", "")
	if err != nil {
		t.Fatal(err)
	}

	controller, err := prepareController(clientset, ns, keyNs, tweakopts, &Flags{SkipRecreate: false}, ssc, keyRegistry)
	if err != nil {
		t.Fatalf("err %v want %v", err, nil)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ss",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("hunter2"),
		},
	}

	cert, err := controller.keyRegistry.getCert()
	if err != nil {
		t.Fatalf("error getting certificate: %v", err)
	}

	ssecret, err := ssv1alpha1.NewSealedSecret(scheme.Codecs, cert.PublicKey.(*rsa.PublicKey), secret)
	if err != nil {
		t.Fatalf("error creating sealed secrets: %v", err)
	}

	// Attacker-controlled, always-failing template paired with the victim's real data.
	ssecret.Spec.Template.Data = map[string]*string{
		"probe": someStr(`{{ fail "attacker-controlled failure" }}`),
	}

	enc, err := prettyEncoder(scheme.Codecs, runtime.ContentTypeJSON, ssv1alpha1.SchemeGroupVersion)
	if err != nil {
		t.Fatalf("unexpected pretty encoding: %v", err)
	}
	data, err := runtime.Encode(enc, ssecret)
	if err != nil {
		t.Fatalf("unexpected encoding the sealed secret: %v", err)
	}

	valid, err := controller.AttemptUnseal(data)
	if err != nil {
		t.Fatalf("AttemptUnseal returned error: %v", err)
	}
	if !valid {
		t.Errorf("AttemptUnseal reported a decryptable secret as invalid because of an unrelated template failure")
	}
}

// TestRotateIgnoresTemplate checks that Rotate never executes spec.template.data as a template.
func TestRotateIgnoresTemplate(t *testing.T) {
	ns := "some-namespace"
	keyNs := "some-key-namespace"
	var tweakopts func(*metav1.ListOptions)
	clientset := fake.NewClientset()
	ssc := ssfake.NewSimpleClientset()
	keyRegistry := testKeyRegister(t, context.Background(), clientset, ns)

	validFor := time.Hour
	cn := "my-cn"
	_, err := keyRegistry.generateKey(context.Background(), validFor, cn, "", "")
	if err != nil {
		t.Fatal(err)
	}

	controller, err := prepareController(clientset, ns, keyNs, tweakopts, &Flags{SkipRecreate: false}, ssc, keyRegistry)
	if err != nil {
		t.Fatalf("err %v want %v", err, nil)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ss",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("hunter2"),
		},
	}

	cert, err := controller.keyRegistry.getCert()
	if err != nil {
		t.Fatalf("error getting certificate: %v", err)
	}

	ssecret, err := ssv1alpha1.NewSealedSecret(scheme.Codecs, cert.PublicKey.(*rsa.PublicKey), secret)
	if err != nil {
		t.Fatalf("error creating sealed secrets: %v", err)
	}

	// Attacker-controlled, always-failing template paired with the victim's real data.
	ssecret.Spec.Template.Data = map[string]*string{
		"probe": someStr(`{{ fail "attacker-controlled failure" }}`),
	}

	enc, err := prettyEncoder(scheme.Codecs, runtime.ContentTypeJSON, ssv1alpha1.SchemeGroupVersion)
	if err != nil {
		t.Fatalf("unexpected pretty encoding: %v", err)
	}
	data, err := runtime.Encode(enc, ssecret)
	if err != nil {
		t.Fatalf("unexpected encoding the sealed secret: %v", err)
	}

	if _, err := controller.Rotate(data); err != nil {
		t.Errorf("Rotate returned error for a decryptable secret because of an unrelated template failure: %v", err)
	}

	// Different guesses against the same ciphertext must behave identically.
	ssecret.Spec.Template.Data = map[string]*string{
		"probe": someStr(`{{ if eq .password "hunter2" }}{{ fail "guessed it" }}{{ end }}`),
	}
	data2, err := runtime.Encode(enc, ssecret)
	if err != nil {
		t.Fatalf("unexpected encoding the sealed secret: %v", err)
	}
	if _, err := controller.Rotate(data2); err != nil {
		t.Errorf("Rotate must not turn template execution into a decryption oracle, got error: %v", err)
	}
}

// prepareController is called once per namespace from parallel goroutines when
// --additional-namespaces is set, so it must be safe to call concurrently.
// Regression test for #2045.
func TestPrepareControllerConcurrently(t *testing.T) {
	ns := "some-namespace"
	keyNs := "some-key-namespace"
	var tweakopts func(*metav1.ListOptions)
	clientset := fake.NewClientset()
	ssc := ssfake.NewSimpleClientset()
	keyRegistry := testKeyRegister(t, context.Background(), clientset, ns)

	const goroutines = 64

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := prepareController(clientset, ns, keyNs, tweakopts, &Flags{}, ssc, keyRegistry); err != nil {
				errs <- err
			}
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("prepareController failed under concurrency: %v", err)
	}
}

// The controller's decoders (scheme.Codecs.UniversalDecoder) rely on the
// SealedSecret types being present in the global scheme. That registration comes
// from the v1alpha1 package init(), which is why NewController does not need to
// repeat it. Guards that invariant.
func TestSealedSecretTypesAreRegisteredGlobally(t *testing.T) {
	for _, kind := range []string{"SealedSecret", "SealedSecretList"} {
		if !scheme.Scheme.Recognizes(ssv1alpha1.SchemeGroupVersion.WithKind(kind)) {
			t.Fatalf("%s is not registered in the global scheme", kind)
		}
	}
}

func TestWatchKeySecretsDynamicDetection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientset := fake.NewClientset()
	var nameSeq int
	clientset.PrependReactor("create", "secrets", func(action ktesting.Action) (bool, runtime.Object, error) {
		ca, ok := action.(ktesting.CreateAction)
		if !ok {
			return false, nil, nil
		}
		sec, ok := ca.GetObject().(*corev1.Secret)
		if ok && sec.Name == "" && sec.GenerateName != "" {
			nameSeq++
			sec.Name = fmt.Sprintf("%s%d", sec.GenerateName, nameSeq)
		}
		return false, nil, nil
	})
	keyRegistry := NewKeyRegistry(clientset, "ns", "prefix", SealedSecretsKeyLabel, 2048)

	informerFactory := informers.NewSharedInformerFactory(clientset, 0)
	kInformer, err := watchKeySecrets(informerFactory, keyRegistry, "CertNotBefore")
	if err != nil {
		t.Fatal(err)
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	go kInformer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, kInformer.HasSynced) {
		t.Fatal("timed out waiting for informer to sync")
	}

	// 1. Generate key and write secret
	key1, cert1, err := generatePrivateKeyAndCert(2048, time.Hour, "cn1")
	if err != nil {
		t.Fatal(err)
	}
	secName1, err := writeKey(ctx, clientset, key1, []*x509.Certificate{cert1}, "ns", SealedSecretsKeyLabel, "prefix-", "", "")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for informer to pick it up
	var latest *rsa.PrivateKey
	for i := 0; i < 20; i++ {
		latest, err = keyRegistry.latestPrivateKey()
		if err == nil && latest != nil && latest.Equal(key1) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if latest == nil || !latest.Equal(key1) {
		t.Fatalf("expected keyRegistry to pick up newly added key secret %s", secName1)
	}

	// 2. Add second newer key via secret update/add
	key2, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cert2, err := crypto.SignKeyWithNotBefore(rand.Reader, key2, time.Now().Add(time.Minute), time.Hour, "cn2")
	if err != nil {
		t.Fatal(err)
	}
	secName2, err := writeKey(ctx, clientset, key2, []*x509.Certificate{cert2}, "ns", SealedSecretsKeyLabel, "prefix-", "", "")
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 20; i++ {
		latest, err = keyRegistry.latestPrivateKey()
		if err == nil && latest != nil && latest.Equal(key2) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if latest == nil || !latest.Equal(key2) {
		t.Fatalf("expected keyRegistry to pick up updated key secret %s", secName2)
	}

	// 3. Delete key2 secret -> should unregister key2 and fallback to key1
	err = clientset.CoreV1().Secrets("ns").Delete(ctx, secName2, metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 20; i++ {
		latest, err = keyRegistry.latestPrivateKey()
		if err == nil && latest != nil && latest.Equal(key1) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if latest == nil || !latest.Equal(key1) {
		t.Fatalf("expected keyRegistry to fallback to key1 after key2 deletion, got: %v", latest)
	}
}

func newTestSealedSecret(ns, name string) *ssv1alpha1.SealedSecret {
	return &ssv1alpha1.SealedSecret{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, UID: "ss-uid"},
	}
}

func newTestOwnedSecret(ss *ssv1alpha1.SealedSecret, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       ss.Namespace,
			Name:            ss.Name,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(ss, ssv1alpha1.SchemeGroupVersion.WithKind("SealedSecret"))},
		},
		Data: data,
	}
}

func newTestQueue(t *testing.T) workqueue.TypedRateLimitingInterface[string] {
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
	t.Cleanup(queue.ShutDown)
	return queue
}

func TestSecretChanged(t *testing.T) {
	base := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"l": "1"}, Annotations: map[string]string{"a": "1"}},
		Data:       map[string][]byte{"foo": []byte("bar")},
		Type:       corev1.SecretTypeOpaque,
	}
	tests := []struct {
		name   string
		modify func(s *corev1.Secret)
		want   bool
	}{
		{name: "no change", modify: func(s *corev1.Secret) {}, want: false},
		{name: "metadata only", modify: func(s *corev1.Secret) {
			s.ResourceVersion = "2"
			s.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "x"}}
		}, want: false},
		{name: "data", modify: func(s *corev1.Secret) { s.Data["foo"] = []byte("oops") }, want: true},
		{name: "data key added", modify: func(s *corev1.Secret) { s.Data["extra"] = []byte("x") }, want: true},
		{name: "type", modify: func(s *corev1.Secret) { s.Type = corev1.SecretTypeTLS }, want: true},
		{name: "labels", modify: func(s *corev1.Secret) { s.Labels["l"] = "2" }, want: false},
		{name: "label added", modify: func(s *corev1.Secret) { s.Labels["extra"] = "x" }, want: false},
		{name: "annotations", modify: func(s *corev1.Secret) { s.Annotations["a"] = "2" }, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updated := base.DeepCopy()
			tc.modify(updated)
			if got := secretChanged(base, updated); got != tc.want {
				t.Errorf("secretChanged() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSecretUpdateHandler(t *testing.T) {
	ss := newTestSealedSecret("ns", "db")
	data := map[string][]byte{"password": []byte("s3cr3t")}
	tests := []struct {
		name          string
		sealedSecrets []runtime.Object
		secret        *corev1.Secret
		modify        func(s *corev1.Secret)
		wantKey       string
	}{
		{
			name:          "owned secret modified",
			sealedSecrets: []runtime.Object{ss},
			secret:        newTestOwnedSecret(ss, data),
			modify:        func(s *corev1.Secret) { s.Data["password"] = []byte("oops") },
			wantKey:       "ns/db",
		},
		{
			name:          "owned secret unchanged",
			sealedSecrets: []runtime.Object{ss},
			secret:        newTestOwnedSecret(ss, data),
			modify:        func(s *corev1.Secret) { s.ResourceVersion = "2" },
		},
		{
			name:          "owned secret labels and annotations modified",
			sealedSecrets: []runtime.Object{ss},
			secret:        newTestOwnedSecret(ss, data),
			modify: func(s *corev1.Secret) {
				s.Labels = map[string]string{"argocd.argoproj.io/instance": "app"}
				s.Annotations = map[string]string{"example.com/note": "added by another tool"}
			},
		},
		{
			name:          "unowned secret modified",
			sealedSecrets: []runtime.Object{ss},
			secret:        &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "db"}, Data: data},
			modify:        func(s *corev1.Secret) { s.Data["password"] = []byte("oops") },
		},
		{
			name:          "managed secret modified",
			sealedSecrets: []runtime.Object{ss},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "ns",
					Name:        "db",
					Annotations: map[string]string{ssv1alpha1.SealedSecretManagedAnnotation: "true"},
				},
				Data: data,
			},
			modify:  func(s *corev1.Secret) { s.Data["password"] = []byte("oops") },
			wantKey: "ns/db",
		},
		{
			name:   "no matching sealed secret",
			secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "unrelated"}, Data: data},
			modify: func(s *corev1.Secret) { s.Data["password"] = []byte("changed") },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			queue := newTestQueue(t)
			handlers := secretEventHandlers(ssfake.NewSimpleClientset(tc.sealedSecrets...), queue)

			oldSecret := tc.secret.DeepCopy()
			newSecret := tc.secret.DeepCopy()
			tc.modify(newSecret)

			handlers.OnUpdate(oldSecret, newSecret)

			if tc.wantKey == "" {
				if got := queue.Len(); got != 0 {
					t.Fatalf("expected no queued item, got %d", got)
				}
				return
			}
			if got := queue.Len(); got != 1 {
				t.Fatalf("expected 1 queued item, got %d", got)
			}
			if key, _ := queue.Get(); key != tc.wantKey {
				t.Errorf("expected key %s, got %q", tc.wantKey, key)
			}
		})
	}
}

func TestSecretDeleteQueuesOwningSealedSecret(t *testing.T) {
	ss := newTestSealedSecret("ns", "db")
	secret := newTestOwnedSecret(ss, map[string][]byte{"password": []byte("s3cr3t")})

	tests := []struct {
		name string
		obj  interface{}
	}{
		{name: "secret", obj: secret},
		{name: "tombstone", obj: cache.DeletedFinalStateUnknown{Key: "ns/db", Obj: secret}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			queue := newTestQueue(t)
			handlers := secretEventHandlers(ssfake.NewSimpleClientset(ss), queue)

			handlers.OnDelete(tc.obj)

			if got := queue.Len(); got != 1 {
				t.Fatalf("expected 1 queued item, got %d", got)
			}
		})
	}
}
