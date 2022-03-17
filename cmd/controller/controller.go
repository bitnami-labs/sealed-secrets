package main

import (
	"context"
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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	ssclientset "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	ssscheme "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned/scheme"
	ssv1alpha1client "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned/typed/sealed-secrets/v1alpha1"
	ssinformer "github.com/bitnami-labs/sealed-secrets/pkg/client/informers/externalversions"
	"github.com/bitnami-labs/sealed-secrets/pkg/multidocyaml"
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
	updateStatus  bool // feature flag that enables updating the status subresource.
}

// NewController returns the main sealed-secrets controller loop.
func NewController(clientset kubernetes.Interface, ssclientset ssclientset.Interface, ssinformer ssinformer.SharedInformerFactory, keyRegistry *KeyRegistry) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	utilruntime.Must(ssscheme.AddToScheme(scheme.Scheme))
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
			if ssecret, ok := obj.(*ssv1alpha1.SealedSecret); ok {
				UnregisterCondition(ssecret)
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

	wait.Until(func() {
		c.runWorker(context.Background())
	}, time.Second, stopCh)

	log.Printf("Shutting down controller")
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextItem(ctx) {
		// continue looping
	}
}

func (c *Controller) processNextItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(key)
	err := c.unseal(ctx, key.(string))
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

func (c *Controller) unseal(ctx context.Context, key string) (unsealErr error) {
	unsealRequestsTotal.Inc()
	obj, exists, err := c.informer.GetIndexer().GetByKey(key)
	if err != nil {
		log.Printf("Error fetching object with key %s from store: %v", key, err)
		unsealErrorsTotal.WithLabelValues("fetch", "").Inc()
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
			err = c.sclient.Secrets(ns).Delete(ctx, name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}

	ssecret := obj.(*ssv1alpha1.SealedSecret)
	log.Printf("Updating %s", key)

	// any exit of this function at this point will cause an update to the status subresource
	// of the SealedSecret custom resource. The return value of the unseal function is available
	// to the deferred function body in the unsealErr named return value (even if explicit return
	// statements are used to return).
	defer func() {
		if err := c.updateSealedSecretStatus(ssecret, unsealErr); err != nil {
			// Non-fatal.  Log and continue.
			log.Printf("Error updating SealedSecret %s status: %v", key, err)
			unsealErrorsTotal.WithLabelValues("status", ssecret.GetNamespace()).Inc()
		} else {
			ObserveCondition(ssecret)
		}
	}()

	newSecret, err := c.attemptUnseal(ssecret)
	if err != nil {
		c.recorder.Eventf(ssecret, corev1.EventTypeWarning, ErrUnsealFailed, "Failed to unseal: %v", err)
		unsealErrorsTotal.WithLabelValues("unseal", ssecret.GetNamespace()).Inc()
		return err
	}

	secret, err := c.sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Get(ctx, newSecret.GetObjectMeta().GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		secret, err = c.sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Create(ctx, newSecret, metav1.CreateOptions{})
	}
	if err != nil {
		c.recorder.Event(ssecret, corev1.EventTypeWarning, ErrUpdateFailed, err.Error())
		unsealErrorsTotal.WithLabelValues("update", ssecret.GetNamespace()).Inc()
		return err
	}

	if !metav1.IsControlledBy(secret, ssecret) && !isAnnotatedToBeManaged(secret) {
		msg := fmt.Sprintf("Resource %q already exists and is not managed by SealedSecret", secret.Name)
		c.recorder.Event(ssecret, corev1.EventTypeWarning, ErrUpdateFailed, msg)
		unsealErrorsTotal.WithLabelValues("unmanaged", ssecret.GetNamespace()).Inc()
		return fmt.Errorf("failed update: %s", msg)
	}

	origSecret := secret
	secret = secret.DeepCopy()

	secret.Data = newSecret.Data
	secret.Type = newSecret.Type
	secret.ObjectMeta.Annotations = newSecret.ObjectMeta.Annotations
	secret.ObjectMeta.OwnerReferences = newSecret.ObjectMeta.OwnerReferences
	secret.ObjectMeta.Labels = newSecret.ObjectMeta.Labels

	if !apiequality.Semantic.DeepEqual(origSecret, secret) {
		_, err = c.sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			c.recorder.Event(ssecret, corev1.EventTypeWarning, ErrUpdateFailed, err.Error())
			unsealErrorsTotal.WithLabelValues("update", ssecret.GetNamespace()).Inc()
			return err
		}
	}

	c.recorder.Event(ssecret, corev1.EventTypeNormal, SuccessUnsealed, "SealedSecret unsealed successfully")
	return nil
}

func (c *Controller) updateSealedSecretStatus(ssecret *ssv1alpha1.SealedSecret, unsealError error) error {
	if !c.updateStatus {
		klog.V(2).Infof("not updating status because updateStatus feature flag not turned on")
		return nil
	}

	if ssecret.Status == nil {
		ssecret.Status = &ssv1alpha1.SealedSecretStatus{}
	}

	// No need to update the status if we already have observed it from the
	// current generation of the resource.
	if ssecret.Status.ObservedGeneration == ssecret.ObjectMeta.Generation {
		return nil
	}

	ssecret.Status.ObservedGeneration = ssecret.ObjectMeta.Generation
	updateSealedSecretsStatusConditions(ssecret.Status, unsealError)

	_, err := c.ssclient.SealedSecrets(ssecret.GetObjectMeta().GetNamespace()).UpdateStatus(ssecret)
	return err
}

func updateSealedSecretsStatusConditions(st *ssv1alpha1.SealedSecretStatus, unsealError error) {
	cond := func() *ssv1alpha1.SealedSecretCondition {
		for i := range st.Conditions {
			if st.Conditions[i].Type == ssv1alpha1.SealedSecretSynced {
				return &st.Conditions[i]
			}
		}
		st.Conditions = append(st.Conditions, ssv1alpha1.SealedSecretCondition{
			Type: ssv1alpha1.SealedSecretSynced,
		})
		return &st.Conditions[len(st.Conditions)-1]
	}()

	var status corev1.ConditionStatus
	if unsealError == nil {
		status = corev1.ConditionTrue
		cond.Message = ""
	} else {
		status = corev1.ConditionFalse
		cond.Message = unsealError.Error()
	}
	cond.LastUpdateTime = metav1.Now()
	if cond.Status != status {
		cond.LastTransitionTime = cond.LastUpdateTime
		cond.Status = status
	}
}

// checks if the annotation equals to "true", and it's case-sensitive
func isAnnotatedToBeManaged(secret *corev1.Secret) bool {
	return secret.Annotations[ssv1alpha1.SealedSecretManagedAnnotation] == "true"
}

// AttemptUnseal tries to unseal a secret.
func (c *Controller) AttemptUnseal(content []byte) (bool, error) {
	if err := multidocyaml.EnsureNotMultiDoc(content); err != nil {
		return false, err
	}

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
		resealedSecret, err := ssv1alpha1.NewSealedSecret(scheme.Codecs, &latestPrivKey.PublicKey, secret)
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
	privateKeys := map[string]*rsa.PrivateKey{}
	for k, v := range keyRegistry.keys {
		privateKeys[k] = v.private
	}
	return ss.Unseal(scheme.Codecs, privateKeys)
}
