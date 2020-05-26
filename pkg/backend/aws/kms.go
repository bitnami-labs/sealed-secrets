package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"

	"net/http"
)

type KMS struct {
	kmsSvc *kms.KMS
	keyID  string
}

func NewKMS(keyID string) (*KMS, error) {

	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("couldn't decrypt using KMS: %v", err)
	}

	svc := kms.New(sess)

	return &KMS{
		kmsSvc: svc,
		keyID:  keyID,
	}, nil

}

func (b *KMS) Encrypt(plaintext, label []byte) ([]byte, error) {

	encryptionContext := make(map[string]*string)
	encryptionContext["kubernetes-secret"] = aws.String(string(label))

	input := &kms.EncryptInput{
		Plaintext:         plaintext,
		EncryptionContext: encryptionContext,
		KeyId:             aws.String(b.keyID),
	}
	result, err := b.kmsSvc.Encrypt(input)
	if err != nil {
		return nil, fmt.Errorf("could encrypt using KMS: %v", err)
	}
	return result.CiphertextBlob, nil
}

func (b *KMS) Decrypt(ciphertext, label []byte) ([]byte, error) {

	encryptionContext := make(map[string]*string)
	encryptionContext["kubernetes-secret"] = aws.String(string(label))

	input := &kms.DecryptInput{
		CiphertextBlob:    ciphertext,
		EncryptionContext: encryptionContext,
		KeyId:             aws.String(b.keyID),
	}
	result, err := b.kmsSvc.Decrypt(input)
	if err != nil {
		return nil, fmt.Errorf("could decrypt using KMS: %v", err)
	}
	return result.Plaintext, nil
}

func (b KMS) Provider(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(b.keyID))
}
