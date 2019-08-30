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

package fake

import (
	v1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// SealedSecrets implements a fake SealedSecretInterface
type SealedSecrets struct {
	Fake *BitnamiV1alpha1
	ns   string
}

var sealedsecretsResource = schema.GroupVersionResource{Group: "bitnami.com", Version: "v1alpha1", Resource: "sealedsecrets"}

var sealedsecretsKind = schema.GroupVersionKind{Group: "bitnami.com", Version: "v1alpha1", Kind: "SealedSecret"}

// Get takes name of the sealedSecret, and returns the corresponding sealedSecret object, and an error if there is any.
func (c *SealedSecrets) Get(name string, options v1.GetOptions) (result *v1alpha1.SealedSecret, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(sealedsecretsResource, c.ns, name), &v1alpha1.SealedSecret{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.SealedSecret), err
}

// List takes label and field selectors, and returns the list of SealedSecrets that match those selectors.
func (c *SealedSecrets) List(opts v1.ListOptions) (result *v1alpha1.SealedSecretList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(sealedsecretsResource, sealedsecretsKind, c.ns, opts), &v1alpha1.SealedSecretList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.SealedSecretList{}
	for _, item := range obj.(*v1alpha1.SealedSecretList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested sealedSecrets.
func (c *SealedSecrets) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(sealedsecretsResource, c.ns, opts))

}

// Create takes the representation of a sealedSecret and creates it.  Returns the server's representation of the sealedSecret, and an error, if there is any.
func (c *SealedSecrets) Create(sealedSecret *v1alpha1.SealedSecret) (result *v1alpha1.SealedSecret, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(sealedsecretsResource, c.ns, sealedSecret), &v1alpha1.SealedSecret{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.SealedSecret), err
}

// Update takes the representation of a sealedSecret and updates it. Returns the server's representation of the sealedSecret, and an error, if there is any.
func (c *SealedSecrets) Update(sealedSecret *v1alpha1.SealedSecret) (result *v1alpha1.SealedSecret, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(sealedsecretsResource, c.ns, sealedSecret), &v1alpha1.SealedSecret{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.SealedSecret), err
}

// Delete takes name of the sealedSecret and deletes it. Returns an error if one occurs.
func (c *SealedSecrets) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(sealedsecretsResource, c.ns, name), &v1alpha1.SealedSecret{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *SealedSecrets) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(sealedsecretsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.SealedSecretList{})
	return err
}

// Patch applies the patch and returns the patched sealedSecret.
func (c *SealedSecrets) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.SealedSecret, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(sealedsecretsResource, c.ns, name, pt, data, subresources...), &v1alpha1.SealedSecret{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.SealedSecret), err
}
