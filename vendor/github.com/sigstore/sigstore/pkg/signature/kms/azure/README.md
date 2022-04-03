# Azure KMS

In order to use Azure KMS with sigstore project you should setup the azure first, the key creation
will be handled in sigstore, however the vault and any needed permission will not and those things need to be configured.

### What I need?

- Create a Resource Group
- In this Resource Group create the Azure KMS
- Configure any custom permission

After that you can use the created vault to generate the key, sign and verify.

For more information check the official Azure Docs: https://azure.microsoft.com/en-us/services/key-vault/
