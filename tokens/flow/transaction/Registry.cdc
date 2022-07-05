import FungibleToken from 0x9a0766d93b6608b7
import %s from %s

transaction() {
    let vaultStoragePath: StoragePath
    let vaultPublicPath:PublicPath 
    let vaultBalancePublicPath:PublicPath 
    prepare(acct: AuthAccount) {
        self.vaultStoragePath= %s
        self.vaultPublicPath= %s
        self.vaultBalancePublicPath= %s

        let tempVault<- %s.createEmptyVault()
		acct.save<@%s.Vault>(<-tempVault, to: self.vaultStoragePath)

        acct.link<&{FungibleToken.Receiver}>(
            self.vaultPublicPath,
            target: self.vaultStoragePath
        )

        // Create a public capability to the stored Vault that only exposes
        // the `balance` field through the `Balance` interface
        //
        acct.link<&%s.Vault{FungibleToken.Balance}>(
            self.vaultBalancePublicPath,
            target: self.vaultStoragePath
        )

    }
}