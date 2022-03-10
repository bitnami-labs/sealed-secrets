/*
Copyright 2022 - Bitnami <containers@bitnami.com>

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

package utils

import (
	"bytes"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

func Test_prettyEncoder(t *testing.T) {
	type args struct {
		contentType string
		gv          runtime.GroupVersioner
	}
	jsonSerializerInfo, _ := runtime.SerializerInfoForMediaType(scheme.Codecs.SupportedMediaTypes(), runtime.ContentTypeJSON)
	yamlSerializerInfo, _ := runtime.SerializerInfoForMediaType(scheme.Codecs.SupportedMediaTypes(), runtime.ContentTypeYAML)
	protobufSerializerInfo, _ := runtime.SerializerInfoForMediaType(scheme.Codecs.SupportedMediaTypes(), runtime.ContentTypeProtobuf)
	tests := []struct {
		name    string
		args    args
		want    runtime.Encoder
		wantErr bool
	}{
		{
			name: "Create a JSON encoder for K8s objects under v1 scheme group",
			args: args{
				contentType: runtime.ContentTypeJSON,
				gv:          v1.SchemeGroupVersion,
			},
			want:    scheme.Codecs.EncoderForVersion(jsonSerializerInfo.PrettySerializer, v1.SchemeGroupVersion),
			wantErr: false,
		},
		{
			name: "Create a YAML encoder for K8s objects under v1 scheme group",
			args: args{
				contentType: runtime.ContentTypeYAML,
				gv:          v1.SchemeGroupVersion,
			},
			want:    scheme.Codecs.EncoderForVersion(yamlSerializerInfo.Serializer, v1.SchemeGroupVersion),
			wantErr: false,
		},
		{
			name: "Create a protobuf encoder for K8s objects under v1 scheme group",
			args: args{
				contentType: runtime.ContentTypeProtobuf,
				gv:          v1.SchemeGroupVersion,
			},
			want:    scheme.Codecs.EncoderForVersion(protobufSerializerInfo.Serializer, v1.SchemeGroupVersion),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		tt := tt // overshadows loop variable tt to be able to run parallel tests
		t.Run(tt.name, func(t *testing.T) {
			got, err := prettyEncoder(tt.args.contentType, tt.args.gv)
			if (err != nil) != tt.wantErr {
				t.Errorf("prettyEncoder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("prettyEncoder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrintResource(t *testing.T) {
	type args struct {
		format string
		obj    runtime.Object
		gv     runtime.GroupVersioner
	}

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Print a valid Secret object in JSON format",
			args: args{
				format: "json",
				obj:    &v1.Secret{},
				gv:     v1.SchemeGroupVersion,
			},
			want: `{
  "kind": "Secret",
  "apiVersion": "v1",
  "metadata": {
    "creationTimestamp": null
  }
}
`,

			wantErr: false,
		},
		{
			name: "Print a valid Secret object in YAML format",
			args: args{
				format: "yaml",
				obj:    &v1.Secret{},
				gv:     v1.SchemeGroupVersion,
			},
			want: `apiVersion: v1
kind: Secret
metadata:
  creationTimestamp: null

`,
			wantErr: false,
		},
		{
			name: "Print a valid Secret object with an invalid format",
			args: args{
				format: "foo",
				obj:    &v1.Secret{},
				gv:     v1.SchemeGroupVersion,
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt // overshadows loop variable tt to be able to run parallel tests
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			if err := PrintResource(tt.args.format, out, tt.args.obj, tt.args.gv); (err != nil) != tt.wantErr {
				t.Errorf("PrintResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOut := out.String(); gotOut != tt.want {
				t.Errorf("PrintResource() = %v, want %v", gotOut, tt.want)
			}
		})
	}
}
