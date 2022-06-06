import FungibleToken from 0xee82856bf20e2aa6
import AnyToken from 0xe03daebed8ca0615
import AnyExampleToken from 0xe03daebed8ca0615
import Router from 0xe03daebed8ca0615

transaction() {
    let minterStoragePath: StoragePath
    let minterPrivatePath: PrivatePath
    let routerRef : Capability<&{AnyToken.IMinter}>
    let mpcStoragePath: StoragePath
    let mpcRef: &Router.Mpc

    prepare(acct: AuthAccount) {
        self.minterStoragePath= /storage/anyExampleTokenMinter
        self.minterPrivatePath=/private/anyExampleTokenMinter
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
 