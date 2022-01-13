# Kubeseal Developer Guide

Kubeseal component is a CLI tool that uses asymmetric crypto to encrypt secrets that only the controller can decrypt.

## Download the Kubeseal source code

```bash
git clone https://github.com/bitnami-labs/sealed-secrets.git $SEALED_SECRETS_DIR
```

The kubeseal sources are located under `cmd/kubeseal/` and use packages from the `pkg` directory.

### Building the `kubeseal` binary

```bash
make kubeseal
```

This builds the `kubeseal` binary in the working directory.

### Running tests

To run the unit tests for `kubeseal` binary:

```bash
make test
```
