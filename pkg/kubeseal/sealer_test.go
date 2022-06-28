package kubeseal

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestSeal(t *testing.T) {
	key, err := ParseKey(strings.NewReader(testCert))
	if err != nil {
		t.Fatalf("Failed to parse test key: %v", err)
	}

	testCases := []struct {
		secret v1.Secret
		scope  ssv1alpha1.SealingScope
		want   ssv1alpha1.SealedSecret // partial object
	}{
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "myns",
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
				StringData: map[string]string{
					"foos": "stringsekret",
				},
			},
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "myns",
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mysecret",
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "default",
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretNamespaceWideAnnotation: "true",
					},
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "default",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretNamespaceWideAnnotation: "true",
					},
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
					},
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "", // <--- we shouldn't force the default namespace for cluster wide secrets ...
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
					},
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "myns",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
					},
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "myns", // <--- ... but we should preserve one if specified.
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
					},
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "",
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			scope: ssv1alpha1.NamespaceWideScope,
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "default",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretNamespaceWideAnnotation: "true",
					},
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "",
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			scope: ssv1alpha1.ClusterWideScope,
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			info, ok := runtime.SerializerInfoForMediaType(scheme.Codecs.SupportedMediaTypes(), runtime.ContentTypeJSON)
			if !ok {
				t.Fatalf("binary can't serialize JSON")
			}
			enc := scheme.Codecs.EncoderForVersion(info.Serializer, v1.SchemeGroupVersion)
			inbuf := bytes.Buffer{}
			if err := enc.Encode(&tc.secret, &inbuf); err != nil {
				t.Fatalf("Error encoding: %v", err)
			}

			t.Logf("input is: %s", inbuf.String())

			outbuf := bytes.Buffer{}
			i := SealInstruction{
				In:                &inbuf,
				Out:               &outbuf,
				Codecs:            scheme.Codecs,
				PubKey:            key,
				Scope:             tc.scope,
				AllowEmptyData:    false,
				DefaultNamespace:  "default",
				OverrideName:      "",
				OverrideNamespace: "",
			}
			if err := Seal(i); err != nil {
				t.Fatalf("seal() returned error: %v", err)
			}

			outBytes := outbuf.Bytes()
			t.Logf("output is %s", outBytes)

			var result ssv1alpha1.SealedSecret
			if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), outBytes, &result); err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}

			smeta := result.GetObjectMeta()
			if got, want := smeta.GetName(), tc.want.GetName(); got != want {
				t.Errorf("got: %q, want: %q", got, want)
			}
			if got, want := smeta.GetNamespace(), tc.want.GetNamespace(); got != want {
				t.Errorf("got: %q, want: %q", got, want)
			}
			if got, want := smeta.GetAnnotations(), tc.want.GetAnnotations(); !cmp.Equal(got, want, cmpopts.EquateEmpty()) {
				t.Errorf("got: %q, want: %q", got, want)
			}

			for n := range tc.secret.Data {
				if len(result.Spec.EncryptedData[n]) < 100 {
					t.Errorf("Encrypted data is implausibly short: %v", result.Spec.EncryptedData[n])
				}
			}
			for n := range tc.secret.StringData {
				if len(result.Spec.EncryptedData[n]) < 100 {
					t.Errorf("Encrypted data is implausibly short: %v", result.Spec.EncryptedData[n])
				}
			}
			// NB: See sealedsecret_test.go for e2e crypto test
		})
	}
}

func TestMergeInto(t *testing.T) {
	pubKey, privKeys := newTestKeyPair(t)

	merge := func(t *testing.T, newSecret, oldSealedSecret []byte) *ssv1alpha1.SealedSecret {
		f, err := ioutil.TempFile("", "*.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(oldSealedSecret); err != nil {
			t.Fatal(err)
		}
		f.Close()

		buf := bytes.NewBuffer(newSecret)
		i := SealMergeIntoInstruction{
			In:             buf,
			Filename:       f.Name(),
			Codecs:         scheme.Codecs,
			PubKey:         pubKey,
			Scope:          ssv1alpha1.DefaultScope,
			AllowEmptyData: false,
		}
		if err := SealMergingInto(i); err != nil {
			t.Fatal(err)
		}

		b, err := ioutil.ReadFile(f.Name())
		if err != nil {
			t.Fatal(err)
		}

		merged, err := DecodeSealedSecret(scheme.Codecs, b)
		if err != nil {
			t.Fatal(err)
		}

		_, err = merged.Unseal(scheme.Codecs, privKeys)
		if err != nil {
			t.Fatal(err)
		}

		return merged
	}

	t.Run("added", func(t *testing.T) {
		merged := merge(t,
			mkTestSecret(t, "foo", "secret1"),
			mkTestSealedSecret(t, pubKey, "bar", "secret2"),
		)

		checkAdded := func(m map[string]string, old, new string) {
			if got, want := len(m), 2; got != want {
				t.Fatalf("got: %d, want: %d", got, want)
			}

			if _, ok := m[old]; !ok {
				t.Fatalf("cannot find expected key")
			}

			if _, ok := m[new]; !ok {
				t.Fatalf("cannot find expected key")
			}
		}

		checkAdded(merged.Spec.EncryptedData, "foo", "bar")
		checkAdded(merged.Spec.Template.Annotations, "foo", "bar")
		checkAdded(merged.Spec.Template.Labels, "foo", "bar")
	})

	t.Run("updated", func(t *testing.T) {
		origSrc := mkTestSealedSecret(t, pubKey, "foo", "secret2")
		orig, err := DecodeSealedSecret(scheme.Codecs, origSrc)
		if err != nil {
			t.Fatal(err)
		}

		merged := merge(t,
			mkTestSecret(t, "foo", "secret1"),
			origSrc,
		)

		checkUpdated := func(before, after map[string]string, key string) {
			if got, want := len(after), 1; got != want {
				t.Fatalf("got: %d, want: %d", got, want)
			}

			if old, new := before[key], after[key]; old == new {
				t.Fatalf("expecting %q and %q to be different", old, new)
			}
		}

		checkUpdated(orig.Spec.EncryptedData, merged.Spec.EncryptedData, "foo")
		checkUpdated(orig.Spec.Template.Annotations, merged.Spec.Template.Annotations, "foo")
		checkUpdated(orig.Spec.Template.Labels, merged.Spec.Template.Labels, "foo")
	})

	t.Run("bad name", func(t *testing.T) {
		// should not fail even if input has a bad secret name because the name in existing existing sealed secret
		// should win (same for namespace).
		// TODO(mkm): test for case with scope mismatch too.
		merge(t,
			mkTestSecret(t, "foo", "secret1", withSecretName("badname"), withSecretNamespace("badns")),
			mkTestSealedSecret(t, pubKey, "bar", "secret2"),
		)
	})
}
