# Crypto details

The controller looks for a cluster-wide private/public key pair on startup, and generates a new 4096 bit (by default) RSA key pair if not found. The key is persisted in a regular Secret in the same namespace as the controller. The public key portion of this (in the form of a self-signed certificate) should be made publicly available to anyone wanting to use SealedSecrets with this cluster. The certificate is printed to the controller log at startup, and available via an HTTP GET to /v1/cert.pem on the controller.

During encryption, each value in the original Secret is symmetrically encrypted using AES-GCM (AES-256) with a randomly-generated single-use 32 byte session key. The session key is then asymmetrically encrypted with the controller's public key using RSA-OAEP (using SHA256), and the original Secret's namespace/name as the OAEP input parameter (aka label). The final output is: 2 byte encrypted session key length || encrypted session key || encrypted Secret.

Note that during decryption by the controller, the SealedSecret's namespace/name is used as the OAEP input parameter, ensuring that the SealedSecret and Secret are tied to the same namespace and name.

When using the namespace-wide scope, the OAEP input (aka label) only contains the namespace and in cluster-wide scope the label is an empty string.
