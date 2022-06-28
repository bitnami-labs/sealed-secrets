package kubeseal

import (
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
)

func PrettyEncoder(codecs runtimeserializer.CodecFactory, mediaType string, gv runtime.GroupVersioner) (runtime.Encoder, error) {
	info, ok := runtime.SerializerInfoForMediaType(codecs.SupportedMediaTypes(), mediaType)
	if !ok {
		return nil, fmt.Errorf("binary can't serialize %s", mediaType)
	}

	prettyEncoder := info.PrettySerializer
	if prettyEncoder == nil {
		prettyEncoder = info.Serializer
	}

	enc := codecs.EncoderForVersion(prettyEncoder, gv)
	return enc, nil
}

func ResourceOutput(out io.Writer, codecs runtimeserializer.CodecFactory, gv runtime.GroupVersioner, obj runtime.Object, outputFormat string) error {

	var contentType string
	switch strings.ToLower(outputFormat) {
	case "json", "":
		contentType = runtime.ContentTypeJSON
	case "yaml":
		contentType = runtime.ContentTypeYAML
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
	prettyEnc, err := PrettyEncoder(codecs, contentType, gv)
	if err != nil {
		return err
	}
	buf, err := runtime.Encode(prettyEnc, obj)
	if err != nil {
		return err
	}
	_, _ = out.Write(buf)
	fmt.Fprint(out, "\n")
	return nil
}
