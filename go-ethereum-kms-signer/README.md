# Go Ethereum KMS Signer
Taken from https://github.com/welthee/go-ethereum-aws-kms-tx-signer. Forking code and reimplementing to ensure the code is secure.

## Usage
This package is compatible with KMS keys that are:
- Asymmetric
- Used for "SIGN_VERIFY"
- With an IAM role that has permission to kms:GetPublicKey and kms:Sign for this key
