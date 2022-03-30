# Crypto details

The controller looks for a cluster-wide private/public key pair on startup, and generates a new 4096 bit (by default) RSA key pair if not found. The key is persisted in a regular Secret in the same namespace as the controller. The public key portion of this (in the form of a self-signed certificate) should be made publicly available to anyone wanting to use SealedSecrets with this cluster. The certificate is printed to the controller log at startup, and available via an HTTP GET to /v1/cert.pem on the controller.

During encryption, each value in the original Secret is symmetrically encrypted using AES-GCM (AES-256) with a randomly-generated single-use 32 byte session key. The session key is then asymmetrically encrypted with the controller's public key using RSA-OAEP (using SHA256), and the original Secret's namespace/name as the OAEP input parameter (aka label). The final output is: 2 byte encrypted session key length || encrypted session key || encrypted Secret.

Note that during decryption by the controller, the SealedSecret's namespace/name is used as the OAEP input parameter, ensuring that the SealedSecret and Secret are tied to the same namespace and name.

When using the namespace-wide scope, the OAEP input (aka label) only contains the namespace and in cluster-wide scope the label is an empty string.

# **Post-quantum cryptography consideration**

## **Entropy source**

### **Analysis**

Even if QRNG (Quantum Random Number Generator) are considered better than PRNG (Pseudo Random Number Generator) in a quantum cryptography context as well as in a non-quantum context, QRNG relies on a quantum mechanical phenomenon. It requires a physical device, therefore QRNG usage is out of Sealed Secrets scope, which will stay on the `crypto/rand` usage.

### **Associated documentation**

[Combining a quantum random number generator and quantum-resistant algorithms into the GnuGPG open-source software](https://doi.org/10.1515/aot-2020-0021)

## **AES-256-GCM**

### **Analysis**

AES-256-GCM is quantum resistant.
Grover algorithm can reduce the bruteforce of the key from 2²⁵⁶ to 2¹²⁸ which is still considered very secure.
Nevertheless, since AES uses unchangeable 128 bits blocks, Grover algorithm can in some cases decrease the complexity of the bruteforce to 2⁶⁴.

### **Recommendations**

AES-256-GCM quantum security is not a concern.
Cases with a bruteforce complexity of 2⁶⁴ are unlikely for Sealed Secret considering how AES is used in the project.
Even assuming that 2⁶⁴ bruteforce is likely, it can still be considered secure today (but not in the long run).
A recommendation is to look for a AES replacement that provide 128 bits post-quantum cryptographic security in any cases, such as ChaCha20-Poly1305. Applying this recommendation is considered low priority.

### **Associated documentation**

[Quantum Security Analysis of AES](https://eprint.iacr.org/2019/272.pdf)

[Critics on AES-256-GCM](https://soatok.blog/2020/05/13/why-aes-gcm-sucks/)

[Security Analysis of ChaCha20-Poly1305 AEAD](https://www.cryptrec.go.jp/exreport/cryptrec-ex-2601-2016.pdf)


## **SHA-256**

### **Analysis**

SHA-256 is quantum resistant.
Grover Algorithm can reduce the bruteforce from 2²⁵⁶ to 2¹²⁸ which is considered very secure.
It is computationally cheaper to use a non-quantum algorithm to generate a collision than to employ a quantum computer.

### **Recommendations**

No recommendations about SHA-256.

### **Associated documentation**
[Cost analysis of hash collisions: Will quantum computers make SHARCS obsolete?](https://cr.yp.to/hash/collisioncost-20090823.pdf)

## **RSA-OAEP**

### **Analysis**

RSA-OAEP, as any RSA algorithm, **is not quantum resistant**.
Shor algorithm can be used to solve in a reasonable time 3 mathematical problems on which RSA cryptography is based on : integer factorization problem, the discrete logarithm problem and the elliptic-curve discrete logarithm problem. Therefore, RSA-OAEP is easily breakable for an attacker with quantum capability.

### **Recommendations**

Replace RSA. This recommendation must be the highest priority regarding the post-quantum security of Sealed Secrets.
There are three serious candidates to use instead of RSA : LMS and XMSS, which are Lattice-based, and McEliece with random Goppa codes, which is code-based and relies on SDP (Syndrome Decoding Problem).
Those three algorithms are serious candidates for RSA replacement and the choice must be done carefully, without forgetting to study other algorithms such as NTRU.

### **Associated documentation**

[LMS](https://datatracker.ietf.org/doc/html/rfc8554)

[XMSS](https://datatracker.ietf.org/doc/html/rfc8391)

[Lattice-based cryptography](https://en.wikipedia.org/wiki/Lattice-based_cryptography)

[McEliece](https://ipnpr.jpl.nasa.gov/progress_report2/42-44/44N.PDF)

[Syndrome Decoding Problem](https://en.wikipedia.org/wiki/Decoding_methods#Syndrome_decoding)

[NIST on post-quantum algorithms](https://csrc.nist.gov/Projects/post-quantum-cryptography/round-3-submissions)

[Quantum-Resistant Cryptography](https://arxiv.org/ftp/arxiv/papers/2112/2112.00399.pdf)
