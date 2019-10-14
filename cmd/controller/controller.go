package main

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	ssclientset "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	ssscheme "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned/scheme"
	ssv1alpha1client "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned/typed/sealed-secrets/v1alpha1"
	ssinformer "github.com/bitnami-labs/sealed-secrets/pkg/client/informers/externalversions"
)

const (
	maxRetries = 5

	// SuccessUnsealed is used as part of the Event 'reason' when
	// a SealedSecret is unsealed successfully.
	SuccessUnsealed = "Unsealed"

	// ErrUpdateFailed is used as part of the Event 'reason' when
	// a SealedSecret fails to update the target Secret for a
	// non-cryptography reason. Typically this is due to API I/O
	// or RBAC issues.
	ErrUpdateFailed = "ErrUpdateFailed"

	// ErrUnsealFailed is used as part of the Event 'reason' when a
	// SealedSecret fails the unsealing process.  Typically this
	// is because it is encrypted with the wrong key or has been
	// renamed from its original namespace/name.
	ErrUnsealFailed = "ErrUnsealFailed"
)

// Controller implements the main sealed-secrets-controller loop.
type Controller struct {
	queue       workqueue.RateLimitingInterface
	informer    cache.SharedIndexInformer
	sclient     v1.SecretsGetter
	ssclient    ssv1alpha1client.SealedSecretsGetter
	recorder    record.EventRecorder
	keyRegistry *KeyRegistry

	oldGCBehavior bool // feature flag to revert to old behavior where we delete the secrets instead of relying on owners reference.
}

func unseal(sclient v1.SecretsGetter, codecs runtimeserializer.CodecFactory, keyRegistry *KeyRegistry, ssecret *ssv1alpha1.SealedSecret) error {
	// Important: Be careful not to reveal the namespace/name of
	// the *decrypted* Secret (or any other detail) in error/log
	// messages.

	objName := fmt.Sprintf("%s/%s", ssecret.GetObjectMeta().GetNamespace(), ssecret.GetObjectMeta().GetName())
	log.Printf("Updating %s", objName)

	secret, err := attemptUnseal(ssecret, keyRegistry)
	if err != nil {
		// TODO: Add error event
		return err
	}

	_, err = sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Create(secret)
	if err != nil && errors.IsAlreadyExists(err) {
		_, err = sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Update(secret)
	}
	if err != nil {
		// TODO: requeue?
		return err
	}

	log.Printf("Updated %s", objName)
	return nil
}

// NewController returns the main sealed-secrets controller loop.
func NewController(clientset kubernetes.Interface, ssclientset ssclientset.Interface, ssinformer ssinformer.SharedInformerFactory, keyRegistry *KeyRegistry) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	ssscheme.AddToScheme(scheme.Scheme)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Printf)
	eventBroadcaster.StartRecordingToSink(&v1.EventSinkImpl{Interface: clientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "sealed-secrets"})

	informer := ssinformer.Bitnami().V1alpha1().
		SealedSecrets().
		Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	})

	return &Controller{
		informer:    informer,
		queue:       queue,
		sclient:     clientset.CoreV1(),
		ssclient:    ssclientset.BitnamiV1alpha1(),
		recorder:    recorder,
		keyRegistry: keyRegistry,
	}
}

// HasSynced returns true once this controller has completed an
// initial resource listing
func (c *Controller) HasSynced() bool {
	return c.informer.HasSynced()
}

// LastSyncResourceVersion is the resource version observed when last
// synced with the underlying store. The value returned is not
// synchronized with access to the underlying store and is not
// thread-safe.
func (c *Controller) LastSyncResourceVersion() string {
	return c.informer.LastSyncResourceVersion()
}

// Run begins processing items, and will continue until a value is
// sent down stopCh.  It's an error to call Run more than once.  Run
// blocks; call via go.
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	defer c.queue.ShutDown()

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	wait.Until(c.runWorker, time.Second, stopCh)

	log.Printf("Shutting down controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
		// continue looping
	}
}

func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(key)
	err := c.unseal(key.(string))
	if err == nil {
		// No error, reset the ratelimit counters
		c.queue.Forget(key)
	} else if c.queue.NumRequeues(key) < maxRetries {
		log.Printf("Error updating %s, will retry: %v", key, err)
		c.queue.AddRateLimited(key)
	} else {
		// err != nil and too many retries
		log.Printf("Error updating %s, giving up: %v", key, err)
		c.queue.Forget(key)
		utilruntime.HandleError(err)
	}

	return true
}

func (c *Controller) unseal(key string) error {
	obj, exists, err := c.informer.GetIndexer().GetByKey(key)
	if err != nil {
		log.Printf("Error fetching object with key %s from store: %v", key, err)
		return err
	}

	if !exists {
		// the dependent secret will be GC: by k8s itself, see:
		// https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#owners-and-dependents

		// TODO: remove this feature flag in a subsequent release.
		if c.oldGCBehavior {
			log.Printf("SealedSecret %s has gone, deleting Secret", key)
			ns, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				return err
			}
			err = c.sclient.Secrets(ns).Delete(name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}

	ssecret := obj.(*ssv1alpha1.SealedSecret)
	log.Printf("Updating %s", key)

	newSecret, err := c.attemptUnseal(ssecret)
	if err != nil {
		c.recorder.Eventf(ssecret, corev1.EventTypeWarning, ErrUnsealFailed, "Failed to unseal: %v", err)
		return err
	}

	secret, err := c.sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Get(newSecret.GetObjectMeta().GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		secret, err = c.sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Create(newSecret)
	}
	if err != nil {
		c.recorder.Event(ssecret, corev1.EventTypeWarning, ErrUpdateFailed, err.Error())
		return err
	}

	if !metav1.IsControlledBy(secret, ssecret) {
		msg := fmt.Sprintf("Resource %q already exists and is not managed by SealedSecret", secret.Name)
		c.recorder.Event(ssecret, corev1.EventTypeWarning, ErrUpdateFailed, msg)
		return fmt.Errorf("failed update: %s", msg)
	}

	origSecret := secret
	secret = secret.DeepCopy()

	secret.Data = newSecret.Data
	secret.Type = newSecret.Type
	secret.ObjectMeta.Annotations = newSecret.ObjectMeta.Annotations
	secret.ObjectMeta.Labels = newSecret.ObjectMeta.Labels

	if !apiequality.Semantic.DeepEqual(origSecret, secret) {
		secret, err = c.sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Update(secret)
		if err != nil {
			c.recorder.Event(ssecret, corev1.EventTypeWarning, ErrUpdateFailed, err.Error())
			return err
		}
	}

	err = c.updateSealedSecretStatus(ssecret, secret)
	if err != nil {
		// Non-fatal.  Log and continue.
		log.Printf("Error updating SealedSecret %s status: %v", key, err)
	}

	c.recorder.Event(ssecret, corev1.EventTypeNormal, SuccessUnsealed, "SealedSecret unsealed successfully")
	return nil
}

func (c *Controller) updateSealedSecretStatus(ssecret *ssv1alpha1.SealedSecret, secret *corev1.Secret) error {
	ssecret = ssecret.DeepCopy()

	ssecret.Status.ObservedGeneration = secret.ObjectMeta.Generation

	// TODO: Use UpdateStatus when k8s CustomResourceSubresources
	// feature is widespread.
	var err error
	ssecret, err = c.ssclient.SealedSecrets(ssecret.GetObjectMeta().GetNamespace()).Update(ssecret)
	return err
}

func (c *Controller) updateSecret(newSecret *corev1.Secret) (*corev1.Secret, error) {
	existingSecret, err := c.sclient.Secrets(newSecret.GetObjectMeta().GetNamespace()).Get(newSecret.GetObjectMeta().GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to read existing secret: %s", err)
	}
	existingSecret = existingSecret.DeepCopy()
	existingSecret.Data = newSecret.Data

	c.updateOwnerReferences(existingSecret, newSecret)

	return existingSecret, nil
}

func (c *Controller) updateOwnerReferences(existing, new *corev1.Secret) {
	ownerRefs := existing.GetOwnerReferences()

	for _, newRef := range new.GetOwnerReferences() {
		found := false
		for _, ref := range ownerRefs {
			if newRef.UID == ref.UID {
				found = true
				break
			}
		}
		if !found {
			ownerRefs = append(ownerRefs, newRef)
		}
	}
	existing.SetOwnerReferences(ownerRefs)
}

// AttemptUnseal tries to unseal a secret.
func (c *Controller) AttemptUnseal(content []byte) (bool, error) {
	object, err := runtime.Decode(scheme.Codecs.UniversalDecoder(ssv1alpha1.SchemeGroupVersion), content)
	if err != nil {
		return false, err
	}

	switch s := object.(type) {
	case *ssv1alpha1.SealedSecret:
		if _, err := c.attemptUnseal(s); err != nil {
			return false, nil
		}
		return true, nil
	default:
		return false, fmt.Errorf("Unexpected resource type: %s", s.GetObjectKind().GroupVersionKind().String())
	}
}

// Rotate takes a sealed secret and returns a sealed secret that has been encrypted
// with the latest private key. If the secret is already encrypted with the latest,
// returns the input.
func (c *Controller) Rotate(content []byte) ([]byte, error) {
	object, err := runtime.Decode(scheme.Codecs.UniversalDecoder(ssv1alpha1.SchemeGroupVersion), content)
	if err != nil {
		return nil, err
	}

	switch s := object.(type) {
	case *ssv1alpha1.SealedSecret:
		secret, err := c.attemptUnseal(s)
		if err != nil {
			return nil, fmt.Errorf("Error decrypting secret. %v", err)
		}
		latestPrivKey := c.keyRegistry.latestPrivateKey()
		resealedSecret, err := ssv1alpha1.NewSealedSecret(scheme.Codecs, "cert", &latestPrivKey.PublicKey, secret)
		if err != nil {
			return nil, fmt.Errorf("Error creating new sealed secret. %v", err)
		}
		data, err := json.Marshal(resealedSecret)
		if err != nil {
			return nil, fmt.Errorf("Error marshalling new secret to json. %v", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("Unexpected resource type: %s", s.GetObjectKind().GroupVersionKind().String())
	}
}

func (c *Controller) attemptUnseal(ss *ssv1alpha1.SealedSecret) (*corev1.Secret, error) {
	return attemptUnseal(ss, c.keyRegistry)
}

func attemptUnseal(ss *ssv1alpha1.SealedSecret, keyRegistry *KeyRegistry) (*corev1.Secret, error) {
	annotations := ss.GetObjectMeta().GetAnnotations()
	encType := "cert"
	if annotations["encryption-type"] == "vault" {
		encType = "vault"
	}
	privateKeys := map[string]*rsa.PrivateKey{}
	for k, v := range keyRegistry.keys {
		privateKeys[k] = v.private
	}
	return ss.Unseal(scheme.Codecs, encType, privateKeys)
}
