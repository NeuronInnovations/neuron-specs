# Neuron SDK — Contract Build Automation
#
# Prerequisites:
#   - Foundry (forge): curl -L https://foundry.paradigm.xyz | bash && foundryup
#   - abigen: go install github.com/ethereum/go-ethereum/cmd/abigen@latest
#   - OpenZeppelin: cd contracts && forge install OpenZeppelin/openzeppelin-contracts --no-git

CONTRACTS_DIR := contracts
GO_IMPL_DIR := impl/golang
REGISTRY_BINDINGS := $(GO_IMPL_DIR)/internal/registry/bindings
PAYMENT_BINDINGS := $(GO_IMPL_DIR)/internal/payment/bindings

# Hedera testnet defaults
HEDERA_EVM_RPC ?= https://testnet.hashio.io/api

.PHONY: contracts forge-build forge-test abigen-registry abigen-escrow abigen-all go-build go-test clean demo demo-help demo-relay demo-browser demo-sapient

## Build all contracts
contracts: forge-build abigen-all

## Compile Solidity contracts
forge-build:
	cd $(CONTRACTS_DIR) && forge build

## Run Solidity tests
forge-test:
	cd $(CONTRACTS_DIR) && forge test -vvv

## Generate Go bindings for Identity Registry
abigen-registry: forge-build
	@mkdir -p $(REGISTRY_BINDINGS)
	abigen \
		--abi $(CONTRACTS_DIR)/out/NeuronIdentityRegistry.sol/NeuronIdentityRegistry.abi.json \
		--bin $(CONTRACTS_DIR)/out/NeuronIdentityRegistry.sol/NeuronIdentityRegistry.bin \
		--pkg bindings \
		--type NeuronIdentityRegistry \
		--out $(REGISTRY_BINDINGS)/identity_registry.go

## Generate Go bindings for NeuronEscrow
abigen-escrow: forge-build
	@mkdir -p $(PAYMENT_BINDINGS)
	abigen \
		--abi $(CONTRACTS_DIR)/out/NeuronEscrow.sol/NeuronEscrow.abi.json \
		--bin $(CONTRACTS_DIR)/out/NeuronEscrow.sol/NeuronEscrow.bin \
		--pkg bindings \
		--type NeuronEscrow \
		--out $(PAYMENT_BINDINGS)/escrow.go
	abigen \
		--abi $(CONTRACTS_DIR)/out/TestToken.sol/TestToken.abi.json \
		--bin $(CONTRACTS_DIR)/out/TestToken.sol/TestToken.bin \
		--pkg bindings \
		--type TestToken \
		--out $(PAYMENT_BINDINGS)/test_token.go

## Generate all Go bindings
abigen-all: abigen-registry abigen-escrow

## Build Go SDK
go-build:
	cd $(GO_IMPL_DIR) && go build ./...

## Run Go tests
go-test:
	cd $(GO_IMPL_DIR) && go test ./internal/... && go vet ./internal/...

## Deploy all contracts to Hedera testnet (requires PRIVATE_KEY env var)
deploy-all:
	cd $(CONTRACTS_DIR) && forge script script/Deploy.s.sol:DeployAll \
		--rpc-url $(HEDERA_EVM_RPC) \
		--private-key $(PRIVATE_KEY) \
		--broadcast

## Deploy only Identity Registry
deploy-registry:
	cd $(CONTRACTS_DIR) && forge script script/Deploy.s.sol:DeployRegistry \
		--rpc-url $(HEDERA_EVM_RPC) \
		--private-key $(PRIVATE_KEY) \
		--broadcast

## Deploy only Escrow + TestToken
deploy-escrow:
	cd $(CONTRACTS_DIR) && forge script script/Deploy.s.sol:DeployEscrow \
		--rpc-url $(HEDERA_EVM_RPC) \
		--private-key $(PRIVATE_KEY) \
		--broadcast

## Clean build artifacts
clean:
	cd $(CONTRACTS_DIR) && forge clean
	rm -rf $(REGISTRY_BINDINGS) $(PAYMENT_BINDINGS)

# ─────────────────────────────────────────────────────────────────────
# Demo targets (Phase 2 onboarding automation)
# Walkthroughs: docs/getting-started/
# ─────────────────────────────────────────────────────────────────────

## Run the canonical zero-friction local demo (mock mode, <1s, no infra)
demo:
	cd $(GO_IMPL_DIR) && go run ./cmd/buyer-seller-demo --mode=mock

## List available demo targets
demo-help:
	@echo "Neuron demo targets:"
	@echo ""
	@echo "  make demo            Local mock buyer-seller (canonical; <1s, no infra, no network)"
	@echo "  make demo-relay      Start a Circuit Relay v2 node (interactive; Ctrl-C to stop)"
	@echo "  make demo-browser    Browser <-> Node seller via libp2p WSS (requires Node 20+ and pnpm)"
	@echo ""
	@echo "Step-by-step walkthroughs: docs/getting-started/"
	@echo "Full demo map:            docs/getting-started/README.md#demo-map"

## Start a Circuit Relay v2 node (runs until Ctrl-C)
demo-relay:
	cd $(GO_IMPL_DIR) && go run ./cmd/relay-node

## Browser demo: Node.js seller + Vite dev server (requires Node 20+ and pnpm)
demo-browser:
	cd impl/typescript && pnpm install && pnpm run demo

## Local SAPIENT Remote ID demo (DS240 sim -> bridge -> seller --dials--> buyer -> fid map)
demo-sapient:
	scripts/demo/sapient-rid-demo.sh
