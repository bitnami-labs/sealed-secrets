package controller

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"k8s.io/client-go/informers"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	ssclientset "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	ssscheme "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned/scheme"
	ssv1alpha1client "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned/typed/sealedsecrets/v1alpha1"
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

var (
	// ErrCast happens when a K8s any type cannot be casted to the expected type.
	ErrCast = errors.New("cast error")
)

// Controller implements the main sealed-secrets-controller loop.
type Controller struct {
	queue       workqueue.RateLimitingInterface
	ssInformer  cache.SharedIndexInformer
	sInformer   cache.SharedIndexInformer
	sclient     v1.SecretsGetter
	ssclient    ssv1alpha1client.SealedSecretsGetter
	recorder    record.EventRecorder
	keyRegistry *KeyRegistry

	oldGCBehavior bool // feature flag to revert to old behavior where we delete the secrets instead of relying on owners reference.
	updateStatus  bool // feature flag that enables updating the status subresource.
}

// NewController returns the main sealed-secrets controller loop.
func NewController(clientset kubernetes.Interface, ssclientset ssclientset.Interface, ssinformer ssinformer.SharedInformerFactory, sinformer informers.SharedInformerFactory, keyRegistry *KeyRegistry) (*Controller, error) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	utilruntime.Must(ssscheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(func(format string, args ...interface{}) {
		// Must use Sprintf to ensure slog doesn't interpret args... as key-value pairs
		slog.Info(fmt.Sprintf(format, args...))
	})
	eventBroadcaster.StartRecordingToSink(&v1.EventSinkImpl{Interface: clientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "sealed-secrets"})

	ssInformer, err := watchSealedSecrets(ssinformer, queue)
	if err != nil {
		return nil, err
	}

	var sInformer cache.SharedIndexInformer
	if sinformer != nil {
		sInformer, err = watchSecrets(sinformer, ssclientset, queue)
		if err != nil {
			return nil, err
		}
	}

	return &Controller{
		ssInformer:  ssInformer,
		sInformer:   sInformer,
		queue:       queue,
		sclient:     clientset.CoreV1(),
		ssclient:    ssclientset.BitnamiV1alpha1(),
		recorder:    recorder,
		keyRegistry: keyRegistry,
	}, nil
}

func watchSealedSecrets(ssinformer ssinformer.SharedInformerFactory, queue workqueue.RateLimitingInterface) (cache.SharedIndexInformer, error) {
	ssInformer := ssinformer.Bitnami().V1alpha1().SealedSecrets().Informer()
	_, err := ssInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				if sealedSecretChanged(oldObj, newObj) {
					queue.Add(key)
				} else {
					slog.Info("update suppressed, no changes in spec", "sealed-secret", key)
				}
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
	if err != nil {
		return nil, fmt.Errorf("could not add event handler to sealed secrets informer: %w", err)
	}
	return ssInformer, nil
}

func sealedSecretChanged(oldObj, newObj interface{}) bool {
	oldSealedSecret, err := convertSealedSecret(oldObj)
	if err != nil {
		return true // any conversion error means we assume it might have changed
	}
	newSealedSecret, err := convertSealedSecret(newObj)
	if err != nil {
		return true
	}
	return !reflect.DeepEqual(oldSealedSecret.Spec, newSealedSecret.Spec)
}

func watchSecrets(sinformer informers.SharedInformerFactory, ssclientset ssclientset.Interface, queue workqueue.RateLimitingInterface) (cache.SharedIndexInformer, error) {
	sInformer := sinformer.Core().V1().Secrets().Informer()
	_, err := sInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			skey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				slog.Error("failed to fetch Secret key", "error", err)
				return
			}

			ns, name, err := cache.SplitMetaNamespaceKey(skey)
			if err != nil {
				slog.Error("failed to get namespace and name from key", "secret", skey, "error", err)
				return
			}

			ssecret, err := ssclientset.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				if !k8serrors.IsNotFound(err) {
					slog.Error("failed to get SealedSecret", "secret", skey, "error", err)
				}
				return
			}

			if !metav1.IsControlledBy(obj.(*corev1.Secret), ssecret) && !isAnnotatedToBeManaged(obj.(*corev1.Secret)) {
				return
			}

			sskey, err := cache.MetaNamespaceKeyFunc(ssecret)
			if err != nil {
				slog.Error("failed to fetch SealedSecret key", "secret", skey, "error", err)
				return
			}

			queue.Add(sskey)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not add event handler to secrets informer: %w", err)
	}
	return sInformer, nil
}

// HasSynced returns true once this controller has completed an
// initial resource listing.
func (c *Controller) HasSynced() bool {
	var synced bool
	if c.sInformer == nil {
		synced = c.ssInformer.HasSynced()
	} else {
		synced = c.ssInformer.HasSynced() && c.sInformer.HasSynced()
	}
	return synced
}

// LastSyncResourceVersion is the resource version observed when last
// synced with the underlying store. The value returned is not
// synchronized with access to the underlying store and is not
// thread-safe.
func (c *Controller) LastSyncResourceVersion() string {
	return c.ssInformer.LastSyncResourceVersion()
}

// Run begins processing items, and will continue until a value is
// sent down stopCh.  It's an error to call Run more than once.  Run
// blocks; call via go.
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	defer c.queue.ShutDown()

	go c.ssInformer.Run(stopCh)
	if c.sInformer != nil {
		go c.sInformer.Run(stopCh)
	}

	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	wait.Until(func() {
		c.runWorker(context.Background())
	}, time.Second, stopCh)

	slog.Error("Shutting down controller")
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
	} else if isImmutableError(err) {
		// Do not retry updating immutable fields of an immutable secret
		slog.Error(formatImmutableError(key.(string)))
		c.queue.Forget(key)
		utilruntime.HandleError(err)
	} else if c.queue.NumRequeues(key) < maxRetries {
		slog.Error("Error updating, will retry", "key", key, "error", err)
		c.queue.AddRateLimited(key)
	} else {
		// err != nil and too many retries
		slog.Error("Error updating, giving up", "key", key, "error", err)
		c.queue.Forget(key)
		utilruntime.HandleError(err)
	}

	return true
}

func (c *Controller) unseal(ctx context.Context, key string) (unsealErr error) {
	unsealRequestsTotal.Inc()
	obj, exists, err := c.ssInformer.GetIndexer().GetByKey(key)
	if err != nil {
		slog.Error("Error fetching object from store", "key", key, "error", err)
		unsealErrorsTotal.WithLabelValues("fetch", "").Inc()
		return err
	}

	if !exists {
		// the dependent secret will be GC: by k8s itself, see:
		// https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#owners-and-dependents

		// TODO: remove this feature flag in a subsequent release.
		if c.oldGCBehavior {
			slog.Info("SealedSecret has gone, deleting Secret", "sealed-secret", key)
			ns, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				return err
			}
			err = c.sclient.Secrets(ns).Delete(ctx, name, metav1.DeleteOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}

	ssecret, err := convertSealedSecret(obj)
	if err != nil {
		return err
	}
	slog.Info("Updating", "key", key)

	// any exit of this function at this point will cause an update to the status subresource
	// of the SealedSecret custom resource. The return value of the unseal function is available
	// to the deferred function body in the unsealErr named return value (even if explicit return
	// statements are used to return).
	defer func() {
		if err := c.updateSealedSecretStatus(ssecret, unsealErr); err != nil {
			// Non-fatal.  Log and continue.
			slog.Error("Error updating SealedSecret status", "sealed-secret", key, "error", err)
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
	if k8serrors.IsNotFound(err) {
		secret, err = c.sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Create(ctx, newSecret, metav1.CreateOptions{})
	}
	if err != nil {
		c.recorder.Event(ssecret, corev1.EventTypeWarning, ErrUpdateFailed, err.Error())
		unsealErrorsTotal.WithLabelValues("update", ssecret.GetNamespace()).Inc()
		return err
	}

	if !metav1.IsControlledBy(secret, ssecret) && !isAnnotatedToBeManaged(secret) && !isAnnotatedToBePatched(secret) {
		msg := fmt.Sprintf("Resource %q already exists and is not managed by SealedSecret", secret.Name)
		c.recorder.Event(ssecret, corev1.EventTypeWarning, ErrUpdateFailed, msg)
		unsealErrorsTotal.WithLabelValues("unmanaged", ssecret.GetNamespace()).Inc()
		return fmt.Errorf("failed update: %s", msg)
	}

	origSecret := secret
	secret = secret.DeepCopy()

	if isAnnotatedToBePatched(secret) {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}

		for k, v := range newSecret.Data {
			secret.Data[k] = v
		}

		if secret.ObjectMeta.Labels == nil {
			secret.ObjectMeta.Labels = make(map[string]string)
		}

		for k, v := range newSecret.ObjectMeta.Labels {
			secret.ObjectMeta.Labels[k] = v
		}

		for k, v := range newSecret.ObjectMeta.Annotations {
			secret.ObjectMeta.Annotations[k] = v
		}

		if isAnnotatedToBeManaged(secret) {
			secret.ObjectMeta.OwnerReferences = newSecret.ObjectMeta.OwnerReferences
		}
	} else {
		secret.Data = newSecret.Data
		secret.ObjectMeta.Annotations = newSecret.ObjectMeta.Annotations
		secret.ObjectMeta.Labels = newSecret.ObjectMeta.Labels
		secret.ObjectMeta.OwnerReferences = newSecret.ObjectMeta.OwnerReferences
	}

	secret.Type = newSecret.Type

	if !apiequality.Semantic.DeepEqual(origSecret, secret) {
		_, err = c.sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			var message = err.Error()
			if isImmutableError(err) {
				message = formatImmutableError(key)
			}

			c.recorder.Event(ssecret, corev1.EventTypeWarning, ErrUpdateFailed, message)
			unsealErrorsTotal.WithLabelValues("update", ssecret.GetNamespace()).Inc()
			return err
		}
	}

	c.recorder.Event(ssecret, corev1.EventTypeNormal, SuccessUnsealed, "SealedSecret unsealed successfully")
	return nil
}

func convertSealedSecret(obj any) (*ssv1alpha1.SealedSecret, error) {
	sealedSecret, ok := (obj).(*ssv1alpha1.SealedSecret)
	if !ok {
		return nil, fmt.Errorf("%w: failed to cast %v into SealedSecret", ErrCast, obj)
	}
	if sealedSecret.APIVersion == "" || sealedSecret.Kind == "" {
		// https://github.com/operator-framework/operator-sdk/issues/727
		gv := schema.GroupVersion{Group: ssv1alpha1.GroupName, Version: "v1alpha1"}
		gvk := gv.WithKind("SealedSecret")
		sealedSecret.APIVersion = gvk.GroupVersion().String()
		sealedSecret.Kind = gvk.Kind
	}
	return sealedSecret, nil
}

func (c *Controller) updateSealedSecretStatus(ssecret *ssv1alpha1.SealedSecret, unsealError error) error {
	if !c.updateStatus {
		klog.V(2).Infof("not updating status because updateStatus feature flag not turned on")
		return nil
	}

	if ssecret.Status == nil {
		ssecret.Status = &ssv1alpha1.SealedSecretStatus{}
	}

	updatedRequired := updateSealedSecretsStatusConditions(ssecret.Status, unsealError)
	if updatedRequired || (ssecret.Status.ObservedGeneration != ssecret.ObjectMeta.Generation) {
		ssecret.Status.ObservedGeneration = ssecret.ObjectMeta.Generation
		_, err := c.ssclient.SealedSecrets(ssecret.GetObjectMeta().GetNamespace()).UpdateStatus(context.Background(), ssecret, metav1.UpdateOptions{})
		return err
	}

	return nil
}

func updateSealedSecretsStatusConditions(st *ssv1alpha1.SealedSecretStatus, unsealError error) bool {
	var updateRequired bool
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
	// Status has changed, update the transition time and signal that an update is required
	if cond.Status != status {
		cond.LastTransitionTime = cond.LastUpdateTime
		cond.Status = status
		updateRequired = true
	}

	return updateRequired
}

func isAnnotatedToBeManaged(secret *corev1.Secret) bool {
	return secret.Annotations[ssv1alpha1.SealedSecretManagedAnnotation] == "true"
}

func isAnnotatedToBePatched(secret *corev1.Secret) bool {
	return secret.Annotations[ssv1alpha1.SealedSecretPatchAnnotation] == "true"
}

func isImmutableError(err error) bool {
	return strings.HasSuffix(err.Error(), "field is immutable when `immutable` is set")
}

func formatImmutableError(key string) string {
	return fmt.Sprintf("Error updating %s: the target Secret is immutable. Once a Secret is marked as immutable, it is not possible to revert this change nor to mutate the contents of the data field. You can only delete and recreate the Secret.", key)
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
		return false, fmt.Errorf("unexpected resource type: %s", s.GetObjectKind().GroupVersionKind().String())
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
			return nil, fmt.Errorf("error decrypting secret. %v", err)
		}
		latestPrivKey := c.keyRegistry.latestPrivateKey()
		resealedSecret, err := ssv1alpha1.NewSealedSecret(scheme.Codecs, &latestPrivKey.PublicKey, secret)
		if err != nil {
			return nil, fmt.Errorf("error creating new sealed secret. %v", err)
		}
		data, err := json.Marshal(resealedSecret)
		if err != nil {
			return nil, fmt.Errorf("error marshalling new secret to json. %v", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unexpected resource type: %s", s.GetObjectKind().GroupVersionKind().String())
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
