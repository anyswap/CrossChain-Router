# Starknet ENV Setup

## 1. Install **Python3.9**
```bash
# Linux
sudo apt install python3.9
# MacOS
brew install python@3.9
```
## 2. Install gmp
```bash
# Linux
sudo apt install -y libgmp3-dev
# MacOS
brew install gmp 
```

If no error message popped up, go to [Step 3. Create project folder](#create-project)

If you have any trouble installing gmp on your Apple M1 computer, here’s a list of potential solutions. 

- Completely remove Python and re-install it.
```bash
# Completely remove python
＄ (base) Username:~ cd Library
sudo rm -rf Python
sudo rm -rf “/Applications/Python”
sudo rm -rf /Library/Frameworks/Python.framework
sudo rm -rf /usr/local/bin/python

# install python3.9
brew install python@3.9
```
- Or, run the following command
```
CFLAGS=-I`brew --prefix gmp`/include LDFLAGS=-L`brew --prefix gmp`/lib pip3 install ecdsa fastecdsa sympy`
```
- Or, use Rosseta homebrew to install gmp
```bash
# open rosetta bash
arch -x86_64 /bin/bash

# install brew on x86 arch
arch -x86_64 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install.sh)"

# Use the x86 brew to install gmp by running 
/usr/local/bin/brew install gmp

# Use the x86 brew to link gmp by running 
/usr/local/bin/brew link gmp
```
- Or, run a dockerized Ubuntu:
```bash
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

docker --version

sudo docker image ls
sudo docker container ls

sudo docker run arm64v8/hello-world

sudo docker image ls
sudo docker container ls -a 
```
Ref. https://github.com/OpenZeppelin/nile/issues/22

## 3. Create project
Create a directory for your project, then cd into it and create a Python virtual environment.
```bash
mkdir project
cd project
python3 -m venv env
source env/bin/activate
```

## 4. Deploy an account
```bash
export STARKNET_NETWORK=alpha-goerli # testnet
export STARKNET_WALLET=starkware.starknet.wallets.open_zeppelin.OpenZeppelinAccount

starknet new_account
```

Output will be like:
```text
Account address: 0x05eb5f19dffbe783fa29b4492061f1c7ed4e979bb9b3006d5853d127a5135685
Public key: 0x00f5af78ae075b657216918526a75ff464411ec9c6c1c7b036c22981bf10efc4
Move the appropriate amount of funds to the account, and then deploy the account
by invoking the 'starknet deploy_account' command.

NOTE: This is a modified version of the OpenZeppelin account contract. The signature is computed
differently.
```

Transfer some (Test) ETH to `Account address` then run:
```bash
starknet deploy_account
```

Wait it to be approved on Starknet. Check tx status:
```bash
starknet tx_status --hash 0xTransactionHash
```


## 5. Replace account public key with MPC public key 
```bash
starknet invoke --address ACCOUNT_ADDRESS --function set_public_key --inputs <MPC_PUBLIC_KEY>  --abi PATH_TO/CrossChain-Router/tokens/starknet/artifacts/abi/account.json 
```

Check result:
```bash
starknet tx_status --hash 0xTransactionHash
# Or
starknet invoke --address ACCOUNT_ADDRESS --function get_public_key --abi PATH_TO/CrossChain-Router/tokens/starknet/artifacts/abi/account.json
```
Now MPC public key has associated with the deployed account address.