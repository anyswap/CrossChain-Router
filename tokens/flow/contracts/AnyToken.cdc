import FungibleToken from 0x9a0766d93b6608b7

pub contract interface AnyToken {

    // pub func underlyingDeposit
    // pub func underlyingWithdraw
    pub resource interface IMinter{
        pub fun mint(amount:UFix64):@FungibleToken.Vault {
            post {
                result.balance == amount:
                    "Balance for result must equals to amount from input"
            }
        }

        pub fun burn(from: @FungibleToken.Vault) {
            pre {
                from.balance > 0.0:
                    "Balance for swapOut must be more than zero"
            }
        }
        // withdraw
    }

    pub resource Minter:IMinter{

        pub fun mint(amount: UFix64): @FungibleToken.Vault {
            pre {
                amount > 0.0:
                    "Amount for swapIn must be more than zero"
            }
            post {
                result.balance == amount:
                    "Balance for result must equals to amount from input"
            }
        }

        pub fun burn(from: @FungibleToken.Vault) {
            pre {
                from.balance > 0.0:
                    "Balance for swapOut must be more than zero"
            }
        }
    }

}