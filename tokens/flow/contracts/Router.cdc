import FungibleToken from 0x9a0766d93b6608b7
import AnyToken from %s

pub contract Router {

    pub let chainId:UInt64
    
    access(contract) var anyTokens:{String:Capability<&{AnyToken.IMinter}>}
    access(contract) var txs:{String:Bool}
    pub event LogSwapOut(
        token:String,
        to:String,
        amount:UFix64,
        fromChainId:UInt64,
        toChainId:UInt64
    )

    pub event LogSwapIn(
        tx:String,
        token:String,
        to:Address,
        amount:UFix64,
        fromChainId:UInt64,
        toChainId:UInt64
    )

    pub fun swapOut(token:String,to:String,toChainId:UInt64,value: @FungibleToken.Vault){
        pre {
            Router.anyTokens.containsKey(token):
                "Router not exists for this token"
        }
        let routerForToken=Router.anyTokens[token]!.borrow()??
            panic("get router for capability fails")
        emit LogSwapOut(token:token,to:to,amount:value.balance,fromChainId:self.chainId,toChainId:toChainId)
        routerForToken.burn(from:<-value)
    }

    pub resource Mpc{

        pub fun insertAnyToken(key:String,value:Capability<&{AnyToken.IMinter}>){
            let capability = value.borrow() ?? panic("cannot borrow Capability")
            Router.anyTokens.insert(key:key,value)
        }

        pub fun removeAnyToken(key:String){
            Router.anyTokens.remove(key:key)
        }

        pub fun swapIn(tx:String,token:String,fromChainId:UInt64,amount:UFix64,receivePaths: [Capability<&{FungibleToken.Receiver}>;2]){
            pre {
                Router.anyTokens.containsKey(token):
                    "Router: Capability not exists for this token"
                !Router.txs.containsKey(tx):
                    "Router: tx has exists"
            }
            Router.txs.insert(key: tx, true)
            let routerForToken=Router.anyTokens[token]!.borrow()
                ??panic("get router for capability fails")
            var receiveRef=receivePaths[0].borrow()
                ??panic("get receive for capability fails")
            let swapVault<-routerForToken.mint(amount:amount)
            emit LogSwapIn(tx:tx,token:token,to:receivePaths[0].address,amount:swapVault.balance,fromChainId:fromChainId,toChainId:Router.chainId)
            if (swapVault.getType().identifier!=receiveRef.getType().identifier){
                receiveRef=receivePaths[1].borrow()
                    ??panic("get receive for capability fails")
            }
            receiveRef.deposit(from: <-swapVault)
        }

        pub fun createNewMpc():@Mpc{
            return <- create Mpc()
        }
    }

    pub fun containsAnyToken(token:String):Bool{
        return Router.anyTokens.containsKey(token)
    }

    pub fun containsTx(tx:String):Bool{
        return Router.txs.containsKey(tx)
    }
    
    init() {
        self.chainId=1001179406168
        self.anyTokens={}
        self.txs={}
        let mpc <- create Mpc()
        self.account.save(<-mpc, to: /storage/routerMpc)
    }
}
