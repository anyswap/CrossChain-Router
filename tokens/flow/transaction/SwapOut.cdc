import FungibleToken from 0x9a0766d93b6608b7
import AnyExampleToken from %s
import Router from %s

transaction(token:String,to:String,toChainId:UInt64,value:UFix64) {
    let vaultRef: &{FungibleToken.Provider}
    let vaultStoragePath:StoragePath
    prepare(acct: AuthAccount) {
        self.vaultStoragePath= /storage/exampleTokenVault
        self.vaultRef = acct.borrow<&{FungibleToken.Provider}>(from:self.vaultStoragePath)
                                ?? panic("Could not borrow a reference to the owner's vault")
    }

    execute {
        log("vaultStoragePath:".concat(self.vaultStoragePath.toString()))
        let temporaryVault <- self.vaultRef.withdraw(amount: value)
        Router.swapOut(token:token,to:to,toChainId:toChainId,value:<-temporaryVault)
    }
}
 