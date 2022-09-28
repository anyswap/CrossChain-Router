# solana router

## github
github: https://github.com/anyswap/CrossChain-Router/tree/feature/solana-support

## router contract
https://github.com/anyswap/router-solana-contract


## router mechanism

1. Swapout from solana to other chain
   
	1.1. Swapout native token `SOL`
	call router contract function `swapout_native`
	```
	pub fn swapout_native(
        ctx: Context<SwapoutNative>,
        to: String,
        lamports: u64,
        to_chainid: u64,
    ) -> Result<()> 

	pub struct SwapoutNative<'info> {
		#[account(mut)]
		pub signer: Signer<'info>,
		#[account(mut, owner = *program_id)]
		pub router_account: Account<'info, RouterAccount>,
		pub system_program: Program<'info, System>,
	}
	```
	1.2. Swapout other token `not anytoken`
	call router contract function `swapout_transfer`
	```
	pub fn swapout_transfer(
        ctx: Context<SwapoutTransfer>,
        to: String,
        amount: u64,
        to_chainid: u64,
    ) -> Result<()>

	pub struct SwapoutTransfer<'info> {
		#[account(mut)]
		pub signer: Signer<'info>,
		#[account(owner = *program_id)]
		pub router_account: Account<'info, RouterAccount>,
		#[account(mut, has_one = mint)]
		pub from: Account<'info, TokenAccount>,
		#[account(mut, has_one = mint)]
		pub to: Account<'info, TokenAccount>,
		#[account(mut, owner = *token_program.key)]
		pub mint: Account<'info, Mint>,
		pub token_program: Program<'info, Token>,
	}
	```

	1.3. Swapout anytoken
	call router contract function `swapout_burn`
	```
	 pub fn swapout_burn(
        ctx: Context<SwapoutBurn>,
        to: String,
        amount: u64,
        to_chainid: u64,
    ) -> Result<()>

	pub struct SwapoutTransfer<'info> {
		#[account(mut)]
		pub signer: Signer<'info>,
		#[account(owner = *program_id)]
		pub router_account: Account<'info, RouterAccount>,
		#[account(mut, has_one = mint)]
		pub from: Account<'info, TokenAccount>,
		#[account(mut, has_one = mint)]
		pub to: Account<'info, TokenAccount>,
		#[account(mut, owner = *token_program.key)]
		pub mint: Account<'info, Mint>,
		pub token_program: Program<'info, Token>,
	}
	```


2. Swapin from other chain to solana

	2.1. Swapin native token `SOL`
	call router contract function `swapin_native`
	```
	pub fn swapin_native(
        ctx: Context<SwapinNative>,
        tx: String,
        lamports: u64,
        from_chainid: u64,
    ) -> Result<()>

	pub struct SwapinNative<'info> {
		#[account(mut)]
		pub mpc: Signer<'info>,
		#[account(mut, owner = *program_id, has_one = mpc @RouterError::OnlyMPC)]
		pub router_account: Account<'info, RouterAccount>,
		#[account(mut)]
		/// CHECK: corss to account
		pub to: AccountInfo<'info>,
		pub system_program: Program<'info, System>,
	}	
	```
	2.2. Swapin other token `not anytoken`
	call router contract function `swapin_transfer`
	```
	pub fn swapin_transfer(
        ctx: Context<SwapinTransfer>,
        tx: String,
        amount: u64,
        from_chainid: u64,
    ) -> Result<()>

	pub struct SwapinTransfer<'info> {
		#[account(mut)]
		pub mpc: Signer<'info>,
		#[account(owner = *program_id, has_one = mpc @RouterError::OnlyMPC)]
		pub router_account: Account<'info, RouterAccount>,
		#[account(mut, has_one = mint)]
		pub from: Account<'info, TokenAccount>,
		#[account(mut, has_one = mint)]
		pub to: Account<'info, TokenAccount>,
		#[account(mut, owner = *token_program.key)]
		pub mint: Account<'info, Mint>,
		pub token_program: Program<'info, Token>,
	}
	```

	2.3. Swapin anytoken
	call router contract function `swapout_burn`
	```
	pub fn swapin_mint(
        ctx: Context<SwapinMint>,
        tx: String,
        amount: u64,
        from_chainid: u64,
    ) -> Result<()>

	pub struct SwapinMint<'info> {
		#[account(mut)]
		pub mpc: Signer<'info>,
		#[account(owner = *program_id, has_one = mpc @RouterError::OnlyMPC)]
		pub router_account: Account<'info, RouterAccount>,
		#[account(mut, has_one = mint)]
		pub to: Account<'info, TokenAccount>,
		#[account(mut, owner = *token_program.key)]
		pub mint: Account<'info, Mint>,
		pub token_program: Program<'info, Token>,
	}
	```


## solana tools

use `-h` option to get help info for each tool

```shell
# convert publickey HexString to solana address
go run tokens/solana/tools/publicKeyToAddress/main.go -h
# change owner for router contract for mpc or privatekey signer
go run tokens/solana/tools/changeMpc/main.go -h
# apply owner for router contract for mpc or privatekey signer
go run tokens/solana/tools/applyMpc/main.go -h
# issue new token by mpc or privatekey signer
go run tokens/solana/tools/issueToken/main.go -h
# create new token AssociatedTokenAccount for mpc in router contract to hold new token asset
go run tokens/solana/tools/createRouterOwnerATA/main.go -h
# gen new solana account
go run tokens/solana/tools/genAccount/main.go -h
# enable or disable swap function 
go run tokens/solana/tools/enableSwap/main.go -h
```

```shell
Example:
go run tokens/solana/tools/publicKeyToAddress/main.go -p 0xedbe0d03d8022012a03d5535e8677681dbbd9bbd130a3593388a61454129f5c294
# output
address: DnsySaKza7ggR6RoviWNWb6WGLg6aKtmYo9dbeuhjQoV
```

## about solana 
API：https://docs.solana.com/developing/clients/jsonrpc-api
cookbook: https://solanacookbook.com/references/token.html#how-to-create-a-token-account
contract sdk: https://github.com/coral-xyz/anchor

### notice
solana use ED25591 algorithm to create account

> devnet  
http:  https://api.devnet.solana.com
chain_id:  245022926

> testnet  
http:  https://api.testnet.solana.com  
chain_id:  245022940

> mainnet  
http: https://api.mainnet-beta.solana.com
chain_id: 245022934


### js sdk
web3: https://solana-labs.github.io/solana-web3.js/index.html
call contract: https://www.npmjs.com/package/@project-serum/anchor

after run `anchor build` (@See https://github.com/anyswap/router-solana-contract)
copy router-solana-contract/target/idl/router.json to project 

```
#Example
const provider = anchor.AnchorProvider.env();
anchor.setProvider(provider);
connection = provider.connection;

const idl = JSON.parse(
	fs.readFileSync("./target/idl/router.json", "utf8")
);
let programId = new anchor.web3.PublicKey("9t6JfntGXehxm7qzLZ71BisNNYCMAiRUgoECvZATVpos");
router_program = new anchor.Program(idl, programId, provider);
```
More Example at https://github.com/anyswap/router-solana-contract/blob/main/app/client.js

### subscribe event logs

```
package main

import (
	"context"

	"github.com/davecgh/go-spew/spew"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

func main() {
	client, err := ws.Connect(context.Background(), rpc.MainNetBeta_WS)
	if err != nil {
		panic(err)
	}
	defer client.Close()
	program := solana.MustPublicKeyFromBase58("9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin") // serum

	{
		// Subscribe to log events that mention the provided pubkey:
		sub, err := client.LogsSubscribeMentions(
			program,
			rpc.CommitmentRecent,
		)
		if err != nil {
			panic(err)
		}
		defer sub.Unsubscribe()

		for {
			got, err := sub.Recv()
			if err != nil {
				panic(err)
			}
			spew.Dump(got)
		}
	}
}
```
sdk https://github.com/gagliardetto/solana-go




