/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	v1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	scheme "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// SealedSecretsGetter has a method to return a SealedSecretInterface.
// A group's client should implement this interface.
type SealedSecretsGetter interface {
	SealedSecrets(namespace string) SealedSecretInterface
}

// SealedSecretInterface has methods to work with SealedSecret resources.
type SealedSecretInterface interface {
	Create(*v1alpha1.SealedSecret) (*v1alpha1.SealedSecret, error)
	Update(*v1alpha1.SealedSecret) (*v1alpha1.SealedSecret, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.SealedSecret, error)
	List(opts v1.ListOptions) (*v1alpha1.SealedSecretList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.SealedSecret, err error)
	SealedSecretExpansion
}

// sealedSecrets implements SealedSecretInterface
type sealedSecrets struct {
	client rest.Interface
	ns     string
}

// newSealedSecrets returns a SealedSecrets
func newSealedSecrets(c *BitnamiV1alpha1Client, namespace string) *sealedSecrets {
	return &sealedSecrets{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the sealedSecret, and returns the corresponding sealedSecret object, and an error if there is any.
func (c *sealedSecrets) Get(name string, options v1.GetOptions) (result *v1alpha1.SealedSecret, err error) {
	result = &v1alpha1.SealedSecret{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("sealedsecrets").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of SealedSecrets that match those selectors.
func (c *sealedSecrets) List(opts v1.ListOptions) (result *v1alpha1.SealedSecretList, err error) {
	result = &v1alpha1.SealedSecretList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("sealedsecrets").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested sealedSecrets.
func (c *sealedSecrets) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("sealedsecrets").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a sealedSecret and creates it.  Returns the server's representation of the sealedSecret, and an error, if there is any.
func (c *sealedSecrets) Create(sealedSecret *v1alpha1.SealedSecret) (result *v1alpha1.SealedSecret, err error) {
	result = &v1alpha1.SealedSecret{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("sealedsecrets").
		Body(sealedSecret).
		Do().
		Into(result)
	return
}

// Update takes the representation of a sealedSecret and updates it. Returns the server's representation of the sealedSecret, and an error, if there is any.
func (c *sealedSecrets) Update(sealedSecret *v1alpha1.SealedSecret) (result *v1alpha1.SealedSecret, err error) {
	result = &v1alpha1.SealedSecret{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("sealedsecrets").
		Name(sealedSecret.Name).
		Body(sealedSecret).
		Do().
		Into(result)
	return
}

// Delete takes name of the sealedSecret and deletes it. Returns an error if one occurs.
func (c *sealedSecrets) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("sealedsecrets").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *sealedSecrets) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("sealedsecrets").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched sealedSecret.
func (c *sealedSecrets) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.SealedSecret, err error) {
	result = &v1alpha1.SealedSecret{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("sealedsecrets").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
