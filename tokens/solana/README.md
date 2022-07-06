# solana router specials

## github
github: https://github.com/anyswap/CrossChain-Router/tree/feature/solana-support

## router deploy
https://github.com/anyswap/CrossChain-Router/tree/feature/near#readme

## solana token config

Native token `SOL` : `contract address` is `native`

Token not issued by multichain : `contract version` is `0`

AnyToken issued by multichain :  `contract version` > `0`

## solana public key to solana address

```shell
go run tokens/solana/tools/publicKeyToAddress/main.go -p 0xedbe0d03d8022012a03d5535e8677681dbbd9bbd130a3593388a61454129f5c294
# output
address: DnsySaKza7ggR6RoviWNWb6WGLg6aKtmYo9dbeuhjQoV
```

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

```text
go run tokens/solana/tools/publicKeyToAddress/main.go -h
	convert solana public key to solana address

go run tokens/solana/tools/changeMpc/main.go -h
	change router contract owner by privateKey or mpc

go run tokens/solana/tools/issueToken/main.go -h
	issue Token by privateKey or mpc

go run tokens/solana/tools/createATA/main.go -h
	create ass

```

## about solana 
API：https://docs.solana.com/developing/clients/jsonrpc-api
cookbook: https://solanacookbook.com/references/token.html#how-to-create-a-token-account
Anchor: https://project-serum.github.io/anchor


> devnet  
http:  https://api.devnet.solana.com
chain_id:  245022926

> testnet  
http:  https://api.testnet.solana.com  
chain_id:  245022940

> mainnet  
http: https://api.mainnet-beta.solana.com
chain_id: 245022934

### notice
solana use ED25591 algorithm to create account


