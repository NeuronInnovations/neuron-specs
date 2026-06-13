// Command sapient-agent-explorer is a simple viewer for SAPIENT Agent Cards.
//
// File mode (testnet-free): reads the evidence JSON files a seller wrote in
// local evidence mode (sapient-rid-seller --registry-out) and prints a table —
// agent id, EVM, PeerID, node_id, service, protocol — so a reviewer can see,
// at a glance, the registered agents and their identity bindings. With --json
// it dumps one agent's full Agent Card (the EIP-8004 agentURI) verbatim.
//
// On-chain mode (buyer-side verification): with --registry-address it resolves
// a card straight from an EIP-8004 Identity Registry over JSON-RPC (read-only;
// no key, no gas) by --owner <sellerEVM> or --agent-id <tokenId>, prints the
// card summary, and runs the chain-side verification (node_id ↔ owner binding,
// /sapient/detection/2.0.0 protocol, rid commerce entry; the pubkey-derived
// PeerID check is reported SKIPPED — the chain stores no public key).
//
// It is standalone by design: it touches no shared/public display component
// (it never imports cmd/fid-display).
//
//	sapient-agent-explorer --dir <evidence-dir>
//	sapient-agent-explorer --dir <evidence-dir> --json <agentId>
//	sapient-agent-explorer --registry-address 0x... --owner 0x...
//	sapient-agent-explorer --registry-address 0x... --agent-id 1 --json-card
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"text/tabwriter"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/neuron-sdk/neuron-go-sdk/internal/dapp/sapient"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

const (
	defaultChainID = uint64(296) // Hedera testnet
	defaultRPCURL  = "https://testnet.hashio.io/api"
)

// resolveDeps holds the injection seam for on-chain mode so tests can swap in
// a MemoryRegistryContract instead of dialing an RPC.
type resolveDeps struct {
	// ContractFactory builds the read-only RegistryContract. nil => the
	// default factory (ethclient dial + EVMRegistryContract with nil auth —
	// resolution only ever reads).
	ContractFactory func(ctx context.Context, rpcURL string, addr keylib.EVMAddress) (registry.RegistryContract, error)
}

func defaultResolveFactory(ctx context.Context, rpcURL string, addr keylib.EVMAddress) (registry.RegistryContract, error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", rpcURL, err)
	}
	// nil auth: the explorer is a verifier — read-only calls only (AgentURIOf,
	// OwnerOf, TokenOfOwnerByIndex). No key, no gas.
	return registry.NewEVMRegistryContract(client, common.HexToAddress(addr.Hex()), nil)
}

func main() {
	if err := run(os.Stdout, os.Args[1:]); err != nil {
		log.Fatalf("sapient-agent-explorer: %v", err)
	}
}

func run(out io.Writer, args []string) error {
	return runWithDeps(out, args, resolveDeps{})
}

func runWithDeps(out io.Writer, args []string, deps resolveDeps) error {
	fs := flag.NewFlagSet("sapient-agent-explorer", flag.ContinueOnError)
	dir := fs.String("dir", "", "directory of agent evidence JSON files (sapient-rid-seller --registry-out); default \".\" when no --registry-address")
	jsonID := fs.String("json", "", "file mode: print the full Agent Card JSON (agentURI) for this agentId and exit")
	registryAddr := fs.String("registry-address", "", "on-chain mode: EIP-8004 Identity Registry contract address (read-only resolve)")
	rpcURL := fs.String("rpc-url", "", "on-chain mode: EVM JSON-RPC endpoint (default "+defaultRPCURL+")")
	chainID := fs.Uint64("chain-id", defaultChainID, "on-chain mode: EVM chain id (Hedera testnet = 296)")
	owner := fs.String("owner", "", "on-chain mode: resolve the agent owned by this seller EVM address")
	agentID := fs.String("agent-id", "", "on-chain mode: resolve the agent with this tokenId/agentId")
	jsonCard := fs.Bool("json-card", false, "on-chain mode: also dump the resolved Agent Card JSON verbatim")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *registryAddr != "" {
		if *dir != "" {
			return errors.New("--dir and --registry-address are mutually exclusive (file mode vs on-chain mode)")
		}
		return resolveOnChain(out, onChainArgs{
			registryHex: *registryAddr,
			rpcURL:      *rpcURL,
			chainID:     *chainID,
			ownerHex:    *owner,
			agentID:     *agentID,
			jsonCard:    *jsonCard,
		}, deps)
	}

	evidenceDir := *dir
	if evidenceDir == "" {
		evidenceDir = "."
	}
	agents, err := sapient.LoadEvidenceDir(evidenceDir)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		return fmt.Errorf("no agent evidence (*.json) found in %s", evidenceDir)
	}

	if *jsonID != "" {
		return printCard(out, agents, *jsonID, evidenceDir)
	}
	return writeTable(out, agents)
}

// onChainArgs carries the parsed on-chain mode flags.
type onChainArgs struct {
	registryHex string
	rpcURL      string
	chainID     uint64
	ownerHex    string
	agentID     string
	jsonCard    bool
}

// resolveOnChain looks the agent up in the Identity Registry (by owner EVM or
// by tokenId), prints the card summary, and runs the chain-side verification.
func resolveOnChain(out io.Writer, a onChainArgs, deps resolveDeps) error {
	if (a.ownerHex == "") == (a.agentID == "") {
		return errors.New("on-chain mode requires exactly one of --owner <0x...> or --agent-id <tokenId>")
	}
	regAddr, err := keylib.EVMAddressFromHex(a.registryHex)
	if err != nil {
		return fmt.Errorf("invalid --registry-address %q: %w", a.registryHex, err)
	}
	if a.chainID == 0 {
		return errors.New("--chain-id must be non-zero for on-chain mode")
	}
	rpc := a.rpcURL
	if rpc == "" {
		rpc = defaultRPCURL
	}

	factory := deps.ContractFactory
	if factory == nil {
		factory = defaultResolveFactory
	}
	ctx := context.Background()
	contract, err := factory(ctx, rpc, regAddr)
	if err != nil {
		return fmt.Errorf("build registry contract: %w", err)
	}

	var key registry.LookupKey
	if a.ownerHex != "" {
		ownerAddr, aerr := keylib.EVMAddressFromHex(a.ownerHex)
		if aerr != nil {
			return fmt.Errorf("invalid --owner %q: %w", a.ownerHex, aerr)
		}
		key = registry.ByEVMAddress(ownerAddr)
	} else {
		key = registry.ByExternalID(a.agentID)
	}

	reg, err := registry.LookupRegistration(ctx, regAddr, a.chainID, key, contract)
	if err != nil {
		return fmt.Errorf("lookup registration: %w", err)
	}

	uri := reg.AgentURI()
	ownerEVM := reg.ChildAddress()
	uriJSON, err := uri.ToJSON()
	if err != nil {
		return fmt.Errorf("serialize resolved agentURI: %w", err)
	}

	var peerID, protocol string
	if p2ps := uri.P2PServices(); len(p2ps) > 0 {
		peerID = p2ps[0].PeerID
		protocol = p2ps[0].Protocol
	}

	tokenID := "-"
	if reg.TokenId() != nil {
		tokenID = reg.TokenId().String()
	}
	fmt.Fprintf(out, "registry:  %s (chainId=%d)\n", regAddr.Hex(), a.chainID)
	fmt.Fprintf(out, "agentId:   %s\n", tokenID)
	fmt.Fprintf(out, "owner:     %s\n", ownerEVM.Hex())
	fmt.Fprintf(out, "peerID:    %s\n", dash(peerID))
	fmt.Fprintf(out, "node_id:   %s\n", dash(sapient.ExtensionNodeID(uri)))
	fmt.Fprintf(out, "protocol:  %s\n", dash(protocol))

	fmt.Fprintln(out, "verification:")
	for _, c := range sapient.VerifyResolvedCard(uri, ownerEVM) {
		fmt.Fprintf(out, "  [%s] %s — %s\n", c.Status, c.Name, c.Detail)
	}

	if a.jsonCard {
		var buf bytes.Buffer
		if err := json.Indent(&buf, []byte(uriJSON), "", "  "); err != nil {
			return fmt.Errorf("indent resolved card: %w", err)
		}
		fmt.Fprintln(out, buf.String())
	}
	return nil
}

// printCard finds the agent by id and writes its indented Agent Card JSON.
func printCard(out io.Writer, agents []sapient.AgentEvidence, agentID, dir string) error {
	for _, a := range agents {
		if a.AgentID == agentID {
			var buf bytes.Buffer
			if err := json.Indent(&buf, a.AgentURI, "", "  "); err != nil {
				return fmt.Errorf("indent card for agent %s: %w", agentID, err)
			}
			_, err := fmt.Fprintln(out, buf.String())
			return err
		}
	}
	return fmt.Errorf("agentId %q not found in %s", agentID, dir)
}

// writeTable renders the agents as an aligned table.
func writeTable(out io.Writer, agents []sapient.AgentEvidence) error {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "AGENT ID\tSELLER EVM\tPEER ID\tNODE ID\tSERVICE\tPROTOCOL\tSIM")
	for _, a := range agents {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			dash(a.AgentID), dash(a.SellerEVM), dash(a.PeerID), dash(a.NodeID),
			dash(a.Service), dash(a.Protocol), simMark(a.Simulated))
	}
	return tw.Flush()
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func simMark(simulated bool) string {
	if simulated {
		return "yes"
	}
	return "no"
}
