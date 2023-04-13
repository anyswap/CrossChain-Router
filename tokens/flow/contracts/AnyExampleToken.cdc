import FungibleToken from 0x9a0766d93b6608b7
import %s from %s
import AnyToken from %s
pub contract AnyExampleToken: FungibleToken, AnyToken {

    /// Total supply of AnyExampleTokens in existence
    pub var totalSupply: UFix64

    // Defines token vault storage path
    pub let TokenStoragePath: StoragePath

    // Defines token vault public balance path
    pub let TokenPublicBalancePath: PublicPath

    // Defines token vault public receiver path
    pub let TokenPublicReceiverPath: PublicPath

    // Controls FlowToken vault
    access(contract) let underlyingVault: @FungibleToken.Vault

    access(contract) let underlyingType:Type?
    /// TokensInitialized
    ///
    /// The event that is emitted when the contract is created
    pub event TokensInitialized(initialSupply: UFix64)

    /// TokensWithdrawn
    ///
    /// The event that is emitted when tokens are withdrawn from a Vault
    pub event TokensWithdrawn(amount: UFix64, from: Address?)

    /// TokensDeposited
    ///
    /// The event that is emitted when tokens are deposited to a Vault
    pub event TokensDeposited(amount: UFix64, to: Address?)

    /// TokensMinted
    ///
    /// The event that is emitted when new tokens are minted
    pub event TokensMinted(amount: UFix64)

    /// TokensBurned
    ///
    /// The event that is emitted when tokens are destroyed
    pub event TokensBurned(amount: UFix64)

    pub event MinterCreated()
    /// Vault
    ///
    /// Each user stores an instance of only the Vault in their storage
    /// The functions in the Vault and governed by the pre and post conditions
    /// in FungibleToken when they are called.
    /// The checks happen at runtime whenever a function is called.
    ///
    /// Resources can only be created in the context of the contract that they
    /// are defined in, so there is no way for a malicious user to create Vaults
    /// out of thin air. A special Minter resource needs to be defined to mint
    /// new tokens.
    ///
    pub resource Vault: FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance {

        /// The total balance of this vault
        pub var balance: UFix64

        // initialize the balance at resource creation time
        init(balance: UFix64) {
            self.balance = balance
        }

        /// withdraw
        ///
        /// Function that takes an amount as an argument
        /// and withdraws that amount from the Vault.
        ///
        /// It creates a new temporary Vault that is used to hold
        /// the money that is being transferred. It returns the newly
        /// created Vault to the context that called so it can be deposited
        /// elsewhere.
        ///
        pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
            self.balance = self.balance - amount
            emit TokensWithdrawn(amount: amount, from: self.owner?.address)
            return <-create Vault(balance: amount)
        }

        /// deposit
        ///
        /// Function that takes a Vault object as an argument and adds
        /// its balance to the balance of the owners Vault.
        ///
        /// It is allowed to destroy the sent Vault because the Vault
        /// was a temporary holder of the tokens. The Vault's balance has
        /// been consumed and therefore can be destroyed.
        ///
        pub fun deposit(from: @FungibleToken.Vault) {
            let vault <- from as! @AnyExampleToken.Vault
            self.balance = self.balance + vault.balance
            emit TokensDeposited(amount: vault.balance, to: self.owner?.address)
            vault.balance = 0.0
            destroy vault
        }

        destroy() {
            AnyExampleToken.totalSupply = AnyExampleToken.totalSupply - self.balance
        }
    }

    /// createEmptyVault
    ///
    /// Function that creates a new Vault with a balance of zero
    /// and returns it to the calling context. A user must call this function
    /// and store the returned Vault in their storage in order to allow their
    /// account to be able to receive deposits of this token type.
    ///
    pub fun createEmptyVault(): @Vault {
        return <-create Vault(balance: 0.0)
    }

    pub resource Administrator {

        /// createNewMinter
        ///
        /// Function that creates and returns a new minter resource
        ///
        pub fun createNewMinter(): @Minter {
            emit MinterCreated()
            return <-create Minter()
        }

    }

    /// Minter
    ///
    /// Resource object that token admin accounts can hold to mint new tokens.
    ///
    pub resource Minter:AnyToken.IMinter {

        pub fun mint(amount: UFix64): @FungibleToken.Vault {
            if AnyExampleToken.underlying()==nil{
                AnyExampleToken.totalSupply=AnyExampleToken.totalSupply+amount
                emit TokensWithdrawn(amount: amount, from: self.owner?.address)
                return <-create Vault(balance: amount)
            }else{
                if AnyExampleToken.underlyingVault.balance>=amount{
                    return <- AnyExampleToken.underlyingVault.withdraw(amount:amount)
                }
                AnyExampleToken.totalSupply=AnyExampleToken.totalSupply+amount
                emit TokensWithdrawn(amount: amount, from: self.owner?.address)
                return <-create Vault(balance: amount)
            }
        }

        pub fun burn(from: @FungibleToken.Vault) {
            if AnyExampleToken.underlying()==nil{
                let vault <- from as! @AnyExampleToken.Vault
                emit TokensBurned(amount: vault.balance)
                destroy vault
            }else{
                AnyExampleToken.underlyingVault.deposit(from:<-from)
            }
        }
    }

    pub fun underlying(): String?{
        if AnyExampleToken.underlyingVault==nil{
            return nil;
        }
        return self.underlyingType!.identifier
    }

    pub fun underlyingDeposit(from:@FungibleToken.Vault):@FungibleToken.Vault {
        pre{
            AnyExampleToken.underlying()!=nil:
            "underlying token not exists"
        }
        let tmpVault<- create Vault(balance:from.balance)
        AnyExampleToken.underlyingVault.deposit(from:<-from)
        return <- tmpVault
    }

    pub fun underlyingWithdraw(from:@FungibleToken.Vault):@FungibleToken.Vault{
        pre{
            AnyExampleToken.underlying()!=nil:
            "underlying token not exists"
        }
        let vault <- from as! @AnyExampleToken.Vault
        emit TokensBurned(amount: vault.balance)
        let tmpVault<-AnyExampleToken.underlyingVault.withdraw(amount:vault.balance)
        destroy vault
        return <- tmpVault
    }

    init() {
        self.TokenStoragePath= /storage/anyExampleTokenVault
        self.TokenPublicBalancePath= /public/anyExampleTokenBalance
        self.TokenPublicReceiverPath= /public/anyExampleTokenReceiver
        self.underlyingVault <- %s.createEmptyVault()
        self.underlyingType = Type<@%s.Vault>()
        self.totalSupply=0.0
        assert(self.underlyingType==self.underlyingVault.getType(), message: "underlying type not match")
        // Create the Vault with the total supply of tokens and save it in storage
        let vault <- create Vault(balance: self.totalSupply)
        self.account.save(<-vault, to: self.TokenStoragePath)

        // Create a public capability to the stored Vault that only exposes
        // the `deposit` method through the `Receiver` interface
        self.account.link<&{FungibleToken.Receiver}>(
            self.TokenPublicReceiverPath,
            target: self.TokenStoragePath
        )

        // Create a public capability to the stored Vault that only exposes
        // the `balance` field through the `Balance` interface
        self.account.link<&AnyExampleToken.Vault{FungibleToken.Balance}>(
            self.TokenPublicBalancePath,
            target: self.TokenStoragePath
        )

        let admin <- create Administrator()
        let minter <- admin.createNewMinter()
        self.account.save(<-minter,to: /storage/anyExampleTokenMinter)
        self.account.save(<-admin, to: /storage/anyExampleTokenAdmin)
    }
}
