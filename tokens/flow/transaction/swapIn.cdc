import FungibleToken from 0xee82856bf20e2aa6
import Router from 0xe03daebed8ca0615

transaction(token:String,receiver:Address,fromChainId:UInt64,amount:UFix64,receivePaths:[String]) {
    let mpcRef: &Router.Mpc
    let mpcStoragePath: StoragePath
    let vaultPublicPath:PublicPath 
    let anyVaultPublicPath:PublicPath 
    let receiverRef:Capability<&{FungibleToken.Receiver}>
    let receiverAnyRef:Capability<&{FungibleToken.Receiver}>
    prepare(acct: AuthAccount) {
        self.mpcStoragePath = /storage/routerMpc
        self.vaultPublicPath = PublicPath(identifier: receivePaths[0])!
        self.anyVaultPublicPath = PublicPath(identifier: receivePaths[1])!
        self.mpcRef=acct.borrow<&Router.Mpc>(from:self.mpcStoragePath)
                                ??panic("Could not borrow a reference to the crosschain")
        let recipient=getAccount(receiver)
        self.receiverRef = recipient.getCapability<&{FungibleToken.Receiver}>(self.vaultPublicPath)
        self.receiverAnyRef = recipient.getCapability<&{FungibleToken.Receiver}>(self.anyVaultPublicPath)
    }

    execute {
        self.mpcRef.swapIn(token:token,fromChainId:fromChainId,amount:amount,receivePaths:[self.receiverRef,self.receiverAnyRef])
    }
}
