import FungibleToken from 0x9a0766d93b6608b7
import AnyToken from %s
import AnyExampleToken from %s
import Router from %s

transaction() {
    let minterStoragePath: StoragePath
    let minterPrivatePath: PrivatePath
    let routerRef : Capability<&{AnyToken.IMinter}>
    let mpcStoragePath: StoragePath
    let mpcRef: &Router.Mpc

    prepare(acct: AuthAccount) {
        self.minterStoragePath= /storage/anyExampleTokenMinter
        self.minterPrivatePath= /private/anyExampleTokenMinter
        self.mpcStoragePath=/storage/routerMpc

        acct.link<&{AnyToken.IMinter}>(
            self.minterPrivatePath,
            target: self.minterStoragePath
        )
        self.mpcRef=acct.borrow<&Router.Mpc>(from:self.mpcStoragePath)
                                ?? panic("Could not borrow a reference to the crosschain")
        if !acct.getCapability<&{AnyToken.IMinter}>(self.minterPrivatePath).check() {
            acct.link<&{AnyToken.IMinter}>(self.minterPrivatePath, target: self.minterStoragePath)
        }

        self.routerRef=acct.getCapability<&{AnyToken.IMinter}>(self.minterPrivatePath)
    }

    execute{
        let token=AnyExampleToken.Vault
        self.mpcRef.insertAnyToken(key:token.getType().identifier, value: self.routerRef)
        log("token:".concat(token.getType().identifier))
        log(Router.containsAnyToken(token: token.getType().identifier))
        log("routerRefIdentifier:".concat(self.routerRef.getType().identifier))
    }
}
 