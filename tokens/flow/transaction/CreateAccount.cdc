import FungibleToken from 0x9a0766d93b6608b7
import FlowToken from 0x7e60df042a9c0868
transaction(pubKey:String) {
    let vaultRef: &FlowToken.Vault
    let vaultStoragePath: StoragePath
    let vaultPublicPath:PublicPath 
    let receiverRef:Capability<&{FungibleToken.Receiver}>
    let publicKey:PublicKey
    let hashAlgorithm:HashAlgorithm
    let weight:UFix64
    let deposit:UFix64
    let account:AuthAccount
    prepare(acct: AuthAccount) {
        self.vaultStoragePath= /storage/flowTokenVault
        self.vaultPublicPath= /public/flowTokenReceiver
        self.publicKey=PublicKey(
            publicKey: pubKey.decodeHex(), 
            signatureAlgorithm: SignatureAlgorithm.ECDSA_secp256k1
        )
        self.hashAlgorithm=HashAlgorithm.SHA3_256
        self.weight=1000.0
        self.deposit=100.0
        self.account = AuthAccount(payer: acct)
        
        self.account.keys.add(publicKey:self.publicKey , hashAlgorithm: self.hashAlgorithm, weight: self.weight)

        self.vaultRef = acct.borrow<&FlowToken.Vault>(from: self.vaultStoragePath)
            ?? panic("Could not borrow a reference to the owner's vault")
        let recipient=getAccount(self.account.address)
        self.receiverRef = recipient.getCapability<&{FungibleToken.Receiver}>(self.vaultPublicPath)
    }

    execute {
        let temporaryVault <- self.vaultRef.withdraw(amount: self.deposit)
        let receiver=self.receiverRef.borrow()??panic("get receiver for capability fails")
        receiver.deposit(from:<-temporaryVault)
        log("account:".concat(self.account.address.toString()))
    }
}
 