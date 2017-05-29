package main

import (
	"crypto/rsa"
	"fmt"
	"io"
	"log"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	ssv1alpha1 "github.com/ksonnet/sealed-secrets/apis/v1alpha1"
)

// NewSealedSecretController returns the main sealed-secrets controller loop.
func NewSealedSecretController(clientset kubernetes.Interface, ssclient rest.Interface, rand io.Reader, privKey *rsa.PrivateKey) (cache.Controller, error) {
	informer := cache.NewSharedInformer(
		cache.NewListWatchFromClient(ssclient, ssv1alpha1.SealedSecretPlural, api.NamespaceAll, fields.Everything()),
		&ssv1alpha1.SealedSecret{},
		0, // No periodic resync
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			ssecret := newObj.(*ssv1alpha1.SealedSecret)
			// Important: Be careful not to reveal the
			// namespace/name of the decrypted Secret (or
			// any other detail) in log messages.

			objName := fmt.Sprintf("%s/%s", ssecret.GetObjectMeta().GetNamespace(), ssecret.GetObjectMeta().GetName())

			secret, err := ssecret.Unseal(api.Codecs, rand, privKey)
			if err != nil {
				// TODO: Add error event
				log.Printf("Error unsealing %s: %v", objName, err)
				return
			}

			log.Printf("Updating %s", objName)

			_, err = clientset.Core().Secrets(ssecret.GetObjectMeta().GetNamespace()).Update(secret)
			if err != nil {
				log.Printf("Error creating Secret: %v", err)
				// TODO: requeue?
				return
			}
			log.Printf("Updated %s", objName)
		},
	})

	return informer, nil
}
