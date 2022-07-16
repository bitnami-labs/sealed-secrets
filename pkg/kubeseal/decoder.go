package kubeseal

import (
	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
)

func DecodeSealedSecret(codecs runtimeserializer.CodecFactory, b []byte) (*ssv1alpha1.SealedSecret, error) {
	var ss ssv1alpha1.SealedSecret
	if err := runtime.DecodeInto(codecs.UniversalDecoder(), b, &ss); err != nil {
		return nil, err
	}
	return &ss, nil
}
