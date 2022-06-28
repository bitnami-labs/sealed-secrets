package kubeseal

import (
	"io"
	"io/ioutil"

	"github.com/bitnami-labs/sealed-secrets/pkg/multidocyaml"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func ReadSecret(codec runtime.Decoder, r io.Reader) (*v1.Secret, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if err := multidocyaml.EnsureNotMultiDoc(data); err != nil {
		return nil, err
	}

	var ret v1.Secret
	if err = runtime.DecodeInto(codec, data, &ret); err != nil {
		return nil, err
	}

	return &ret, nil
}
