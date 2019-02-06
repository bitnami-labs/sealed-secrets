package main

import (
	"fmt"
	"log"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	ssinformer "github.com/bitnami-labs/sealed-secrets/pkg/client/informers/externalversions"
)

const maxRetries = 5

// Controller implements the main sealed-secrets-controller loop.
type Controller struct {
	queue       workqueue.RateLimitingInterface
	informer    cache.SharedIndexInformer
	sclient     v1.SecretsGetter
	keyRegistry *KeyRegistry
}

func unseal(sclient v1.SecretsGetter, codecs runtimeserializer.CodecFactory, keyRegistry *KeyRegistry, ssecret *ssv1alpha1.SealedSecret) error {
	// Important: Be careful not to reveal the namespace/name of
	// the *decrypted* Secret (or any other detail) in error/log
	// messages.

	objName := fmt.Sprintf("%s/%s", ssecret.GetObjectMeta().GetNamespace(), ssecret.GetObjectMeta().GetName())
	log.Printf("Updating %s", objName)

	secret, err := ssecret.Unseal(codecs, keyRegistry)
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
func NewController(clientset kubernetes.Interface, ssinformer ssinformer.SharedInformerFactory, keyRegistry *KeyRegistry) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

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
		sclient:     clientset.Core(),
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
		log.Printf("SealedSecret %s has gone, deleting Secret", key)
		ns, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return err
		}
		err = c.sclient.Secrets(ns).Delete(name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	ssecret := obj.(*ssv1alpha1.SealedSecret)
	log.Printf("Updating %s", key)

	secret, err := ssecret.Unseal(scheme.Codecs, c.keyRegistry)
	if err != nil {
		return err
	}

	_, err = c.sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Create(secret)
	if err != nil && errors.IsAlreadyExists(err) {
		_, err = c.sclient.Secrets(ssecret.GetObjectMeta().GetNamespace()).Update(secret)
	}
	return err
}

func (c *Controller) AttemptUnseal(content []byte) (bool, error) {
	object, err := runtime.Decode(scheme.Codecs.UniversalDecoder(ssv1alpha1.SchemeGroupVersion), content)
	if err != nil {
		return false, err
	}

	switch s := object.(type) {
	case *ssv1alpha1.SealedSecret:
		if _, err := s.Unseal(scheme.Codecs, c.keyRegistry); err != nil {
			return false, nil
		}
		return true, nil
	default:
		return false, fmt.Errorf("Unexpected resource type: %s", s.GetObjectKind().GroupVersionKind().String())

	}
}
