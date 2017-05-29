package main

import (
	"crypto/rsa"
	"fmt"
	"io"
	"log"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	ssv1alpha1 "github.com/ksonnet/sealed-secrets/apis/v1alpha1"
)

func unseal(clientset kubernetes.Interface, codecs runtimeserializer.CodecFactory, rnd io.Reader, key *rsa.PrivateKey, ssecret *ssv1alpha1.SealedSecret) error {
	// Important: Be careful not to reveal the namespace/name of
	// the decrypted Secret (or any other detail) in log messages.

	objName := fmt.Sprintf("%s/%s", ssecret.GetObjectMeta().GetNamespace(), ssecret.GetObjectMeta().GetName())
	log.Printf("Updating %s", objName)

	secret, err := ssecret.Unseal(codecs, rnd, key)
	if err != nil {
		// TODO: Add error event
		return err
	}

	_, err = clientset.Core().Secrets(ssecret.GetObjectMeta().GetNamespace()).Create(secret)
	if err != nil && errors.IsAlreadyExists(err) {
		_, err = clientset.Core().Secrets(ssecret.GetObjectMeta().GetNamespace()).Update(secret)
	}
	if err != nil {
		// TODO: requeue?
		return err
	}

	log.Printf("Updated %s", objName)
	return nil
}

// NewSealedSecretController returns the main sealed-secrets controller loop.
func NewSealedSecretController(clientset kubernetes.Interface, ssclient rest.Interface, rand io.Reader, privKey *rsa.PrivateKey) (cache.Controller, error) {
	informer := cache.NewSharedInformer(
		cache.NewListWatchFromClient(ssclient, ssv1alpha1.SealedSecretPlural, api.NamespaceAll, fields.Everything()),
		&ssv1alpha1.SealedSecret{},
		0, // No periodic resync
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ssecret := obj.(*ssv1alpha1.SealedSecret)
			if err := unseal(clientset, api.Codecs, rand, privKey, ssecret); err != nil {
				log.Printf("Error unsealing %s/%s: %v", ssecret.GetObjectMeta().GetNamespace(), ssecret.GetObjectMeta().GetName(), err)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			ssecret := newObj.(*ssv1alpha1.SealedSecret)
			if err := unseal(clientset, api.Codecs, rand, privKey, ssecret); err != nil {
				log.Printf("Error unsealing %s/%s: %v", ssecret.GetObjectMeta().GetNamespace(), ssecret.GetObjectMeta().GetName(), err)
			}
		},
	})

	return informer, nil
}
