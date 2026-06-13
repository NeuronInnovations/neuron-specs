// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// NeuronEscrowMetaData contains all meta data concerning the NeuronEscrow contract.
var NeuronEscrowMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"approveRelease\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"releaseId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"claimRefund\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"createEscrow\",\"inputs\":[{\"name\":\"buyer\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"seller\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"arbiter\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"threshold\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"agreementHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"timeout\",\"type\":\"uint64\",\"internalType\":\"uint64\"}],\"outputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"deposit\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"getBalance\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"available\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getEscrow\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"buyer\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"seller\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"arbiter\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"threshold\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"agreementHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"timeout\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"balance\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"pendingReleaseTotal\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"state\",\"type\":\"uint8\",\"internalType\":\"enumNeuronEscrow.EscrowState\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getRelease\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"releaseId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"recipient\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"evidenceHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"state\",\"type\":\"uint8\",\"internalType\":\"enumNeuronEscrow.ReleaseState\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"requestRelease\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"recipient\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"evidenceHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"releaseId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"withdraw\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"releaseId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"Deposited\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"depositor\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"newBalance\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"EscrowCreated\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"buyer\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"seller\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"token\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"},{\"name\":\"agreementHash\",\"type\":\"bytes32\",\"indexed\":false,\"internalType\":\"bytes32\"},{\"name\":\"timeout\",\"type\":\"uint64\",\"indexed\":false,\"internalType\":\"uint64\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RefundClaimed\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"buyer\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ReleaseApproved\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"releaseId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ReleaseRequested\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"releaseId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"recipient\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"address\"},{\"name\":\"evidenceHash\",\"type\":\"bytes32\",\"indexed\":false,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Withdrawn\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"releaseId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"recipient\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"EscrowNotFound\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"InsufficientBalance\",\"inputs\":[{\"name\":\"requested\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"available\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"InvalidAmount\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"InvalidParticipant\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NotBuyer\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"caller\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"NotParticipant\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"caller\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ReentrancyGuardReentrantCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"ReleaseNotApproved\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"releaseId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"ReleaseNotFound\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"releaseId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"ReleaseNotPending\",\"inputs\":[{\"name\":\"escrowId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"releaseId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"SafeERC20FailedOperation\",\"inputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"TimeoutElapsed\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"TimeoutNotElapsed\",\"inputs\":[{\"name\":\"current\",\"type\":\"uint64\",\"internalType\":\"uint64\"},{\"name\":\"timeout\",\"type\":\"uint64\",\"internalType\":\"uint64\"}]}]",
	Bin: "0x608060405260015f55348015610013575f80fd5b5060017f9b779b17422d0df92223018b32b4d1fa46e071723d6817e2486d003becc55f0055611120806100455f395ff3fe608060405234801561000f575f80fd5b5060043610610090575f3560e01c80636e4a31db116100635780636e4a31db146101055780637d19e59614610118578063a5f95add14610141578063cb00801e14610154578063e2bbb15814610167575f80fd5b80631e010439146100945780631f4eb738146100ba578063441a3e70146100dd5780635b7baf64146100f2575b5f80fd5b6100a76100a2366004610e88565b61017a565b6040519081526020015b60405180910390f35b6100cd6100c8366004610e9f565b6101e1565b6040516100b19493929190610ed3565b6100f06100eb366004610e9f565b610266565b005b6100f0610100366004610e88565b61041d565b6100a7610113366004610f2a565b6105a8565b61012b610126366004610e88565b6107db565b6040516100b19a99989796959493929190610f64565b6100a761014f366004610ff8565b6108a3565b6100f0610162366004610e9f565b610a5f565b6100f0610175366004610e9f565b610bcc565b5f8181526001602052604081205482906001600160a01b03166101b8576040516305b3479760e11b8152600481018290526024015b60405180910390fd5b5f838152600160205260409020600781015460068201546101d99190611086565b949350505050565b5f8281526001602052604081205481908190819086906001600160a01b0316610220576040516305b3479760e11b8152600481018290526024016101af565b5050505f9384525050600160208181526040808520938552600a90930190529120805491810154600282015460039092015492936001600160a01b039091169260ff1690565b61026e610cf7565b5f8281526001602052604090205482906001600160a01b03166102a7576040516305b3479760e11b8152600481018290526024016101af565b5f838152600160209081526040808320858452600a8101909252822080549192909190036102f257604051631da5b77d60e11b815260048101869052602481018590526044016101af565b6001600382015460ff16600281111561030d5761030d610ebf565b1461033557604051633323513560e21b815260048101869052602481018590526044016101af565b60038101805460ff1916600217905580546006830180548291905f9061035c908490611086565b9250508190555080836007015f8282546103769190611086565b909155505060068301545f036103965760088301805460ff191660021790555b600182015460038401546103b7916001600160a01b03918216911683610d12565b60018201546040518281526001600160a01b0390911690869088907fef05a23f979cd8b846e8a62f76d15195e9a92e83a36901eb7eceaa476c69d25c9060200160405180910390a45050505061041960015f805160206110cb83398151915255565b5050565b610425610cf7565b5f8181526001602052604090205481906001600160a01b031661045e576040516305b3479760e11b8152600481018290526024016101af565b5f82815260016020526040902080546001600160a01b0316331461049e5760405163478f059d60e11b8152600481018490523360248201526044016101af565b600581015467ffffffffffffffff164210156104e757600581015460405163d07d421560e01b815267ffffffffffffffff428116600483015290911660248201526044016101af565b60068101545f81900361050d5760405163162908e360e11b815260040160405180910390fd5b5f60068301819055600783015560088201805460ff19166003908117909155825490830154610549916001600160a01b03918216911683610d12565b81546040518281526001600160a01b039091169085907ff3f402280ef0a7905e124aa621b65eaeb2725c343e8b36d398ed78c29daf285c9060200160405180910390a35050506105a560015f805160206110cb83398151915255565b50565b5f6105b1610cf7565b5f8581526001602052604090205485906001600160a01b03166105ea576040516305b3479760e11b8152600481018290526024016101af565b845f0361060a5760405163162908e360e11b815260040160405180910390fd5b5f868152600160208190526040909120908101546001600160a01b03163314801590610643575060028101546001600160a01b03163314155b1561066a57604051633e50052d60e21b8152600481018890523360248201526044016101af565b5f8160070154826006015461067f9190611086565b9050808711156106ac5760405163cf47918160e01b815260048101889052602481018290526044016101af565b600982018054905f6106bd8361109f565b90915550604080516080810182528981526001600160a01b0389811660208084019182528385018b81525f60608601818152888252600a8b0190935295909520845181559151600180840180546001600160a01b031916929095169190911790935593516002808301919091559351600382018054969a509395919490939260ff199092169190849081111561075557610755610ebf565b021790555090505086826007015f82825461077091906110b7565b9091555050604080518881526001600160a01b0388166020820152908101869052849089907fd4e5d6091fb42e83daa719b2a3466be650e5519653d8eb32312fa014c37032179060600160405180910390a35050506101d960015f805160206110cb83398151915255565b5f818152600160205260408120548190819081908190819081908190819081908b906001600160a01b0316610826576040516305b3479760e11b8152600481018290526024016101af565b5050505f98895250506001602081905260409097208054978101546002820154600383015460048401546005850154600686015460078701546008909701546001600160a01b039e8f169f968f169e9586169d509484169b50600160a01b90930467ffffffffffffffff9081169a50919850169550935060ff1690565b5f6108ac610cf7565b6001600160a01b03881615806108c957506001600160a01b038716155b156108e7576040516350a2e21f60e11b815260040160405180910390fd5b6001600160a01b03851661090e576040516350a2e21f60e11b815260040160405180910390fd5b5f8054908061091c8361109f565b909155505f818152600160208190526040822080546001600160a01b03808e166001600160a01b031992831617835582840180548e83169084161790556002830180548d831693169290921790915560038201805467ffffffffffffffff808c16600160a01b026001600160e01b0319909216938d16939093171790556004820188905560058201805491881667ffffffffffffffff199092169190911790556008810180549495509093909160ff199091169083021790555060016009820155604080516001600160a01b0388811682526020820187905267ffffffffffffffff861692820192909252818a16918b169084907fb82e1117edbb466f074004163623779e929f4600d80b8c467760e073b8c1e7b49060600160405180910390a450610a5460015f805160206110cb83398151915255565b979650505050505050565b610a67610cf7565b5f8281526001602052604090205482906001600160a01b0316610aa0576040516305b3479760e11b8152600481018290526024016101af565b5f83815260016020526040902080546001600160a01b03163314801590610ad4575060028101546001600160a01b03163314155b15610afb57604051633e50052d60e21b8152600481018590523360248201526044016101af565b5f838152600a8201602052604081208054909103610b3657604051631da5b77d60e11b815260048101869052602481018590526044016101af565b5f600382015460ff166002811115610b5057610b50610ebf565b14610b7857604051635c6d6a1360e01b815260048101869052602481018590526044016101af565b60038101805460ff19166001179055604051849086907f1ad3f5d5c07752e6a836347b4cd670dbba93b657c0094d4a3d0faaaa3d5ebba8905f90a350505061041960015f805160206110cb83398151915255565b610bd4610cf7565b5f8281526001602052604090205482906001600160a01b0316610c0d576040516305b3479760e11b8152600481018290526024016101af565b815f03610c2d5760405163162908e360e11b815260040160405180910390fd5b5f8381526001602052604090206003810154610c54906001600160a01b0316333086610d4c565b82816006015f828254610c6791906110b7565b909155505f9050600882015460ff166003811115610c8757610c87610ebf565b03610c9c5760088101805460ff191660011790555b6006810154604080518581526020810192909252339186917fad5b4075b97dbf75ad5c78f7afac948e4ae611c4fdf2825e2ce3c6c96925bf3b910160405180910390a3505061041960015f805160206110cb83398151915255565b610cff610d88565b60025f805160206110cb83398151915255565b610d1f8383836001610db9565b610d4757604051635274afe760e01b81526001600160a01b03841660048201526024016101af565b505050565b610d5a848484846001610e1b565b610d8257604051635274afe760e01b81526001600160a01b03851660048201526024016101af565b50505050565b5f805160206110cb83398151915254600203610db757604051633ee5aeb560e01b815260040160405180910390fd5b565b60405163a9059cbb60e01b5f8181526001600160a01b038616600452602485905291602083604481808b5af1925060015f51148316610e0f578383151615610e03573d5f823e3d81fd5b5f873b113d1516831692505b60405250949350505050565b6040516323b872dd60e01b5f8181526001600160a01b038781166004528616602452604485905291602083606481808c5af1925060015f51148316610e77578383151615610e6b573d5f823e3d81fd5b5f883b113d1516831692505b604052505f60605295945050505050565b5f60208284031215610e98575f80fd5b5035919050565b5f8060408385031215610eb0575f80fd5b50508035926020909101359150565b634e487b7160e01b5f52602160045260245ffd5b8481526001600160a01b0384166020820152604081018390526080810160038310610f0057610f00610ebf565b82606083015295945050505050565b80356001600160a01b0381168114610f25575f80fd5b919050565b5f805f8060808587031215610f3d575f80fd5b8435935060208501359250610f5460408601610f0f565b9396929550929360600135925050565b6001600160a01b038b811682528a8116602083015289811660408301528816606082015267ffffffffffffffff878116608083015260a08201879052851660c082015260e081018490526101008101839052610140810160048310610fcb57610fcb610ebf565b826101208301529b9a5050505050505050505050565b803567ffffffffffffffff81168114610f25575f80fd5b5f805f805f805f60e0888a03121561100e575f80fd5b61101788610f0f565b965061102560208901610f0f565b955061103360408901610f0f565b945061104160608901610f0f565b935061104f60808901610fe1565b925060a0880135915061106460c08901610fe1565b905092959891949750929550565b634e487b7160e01b5f52601160045260245ffd5b8181038181111561109957611099611072565b92915050565b5f600182016110b0576110b0611072565b5060010190565b808201808211156110995761109961107256fe9b779b17422d0df92223018b32b4d1fa46e071723d6817e2486d003becc55f00a26469706673582212209e42bbe01481f52e803ce65ba3d316eb7605a864e9016a5f9ec16a5c3812d12464736f6c63430008180033",
}

// NeuronEscrowABI is the input ABI used to generate the binding from.
// Deprecated: Use NeuronEscrowMetaData.ABI instead.
var NeuronEscrowABI = NeuronEscrowMetaData.ABI

// NeuronEscrowBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use NeuronEscrowMetaData.Bin instead.
var NeuronEscrowBin = NeuronEscrowMetaData.Bin

// DeployNeuronEscrow deploys a new Ethereum contract, binding an instance of NeuronEscrow to it.
func DeployNeuronEscrow(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *NeuronEscrow, error) {
	parsed, err := NeuronEscrowMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(NeuronEscrowBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &NeuronEscrow{NeuronEscrowCaller: NeuronEscrowCaller{contract: contract}, NeuronEscrowTransactor: NeuronEscrowTransactor{contract: contract}, NeuronEscrowFilterer: NeuronEscrowFilterer{contract: contract}}, nil
}

// NeuronEscrow is an auto generated Go binding around an Ethereum contract.
type NeuronEscrow struct {
	NeuronEscrowCaller     // Read-only binding to the contract
	NeuronEscrowTransactor // Write-only binding to the contract
	NeuronEscrowFilterer   // Log filterer for contract events
}

// NeuronEscrowCaller is an auto generated read-only Go binding around an Ethereum contract.
type NeuronEscrowCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NeuronEscrowTransactor is an auto generated write-only Go binding around an Ethereum contract.
type NeuronEscrowTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NeuronEscrowFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type NeuronEscrowFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NeuronEscrowSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type NeuronEscrowSession struct {
	Contract     *NeuronEscrow     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// NeuronEscrowCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type NeuronEscrowCallerSession struct {
	Contract *NeuronEscrowCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// NeuronEscrowTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type NeuronEscrowTransactorSession struct {
	Contract     *NeuronEscrowTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// NeuronEscrowRaw is an auto generated low-level Go binding around an Ethereum contract.
type NeuronEscrowRaw struct {
	Contract *NeuronEscrow // Generic contract binding to access the raw methods on
}

// NeuronEscrowCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type NeuronEscrowCallerRaw struct {
	Contract *NeuronEscrowCaller // Generic read-only contract binding to access the raw methods on
}

// NeuronEscrowTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type NeuronEscrowTransactorRaw struct {
	Contract *NeuronEscrowTransactor // Generic write-only contract binding to access the raw methods on
}

// NewNeuronEscrow creates a new instance of NeuronEscrow, bound to a specific deployed contract.
func NewNeuronEscrow(address common.Address, backend bind.ContractBackend) (*NeuronEscrow, error) {
	contract, err := bindNeuronEscrow(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &NeuronEscrow{NeuronEscrowCaller: NeuronEscrowCaller{contract: contract}, NeuronEscrowTransactor: NeuronEscrowTransactor{contract: contract}, NeuronEscrowFilterer: NeuronEscrowFilterer{contract: contract}}, nil
}

// NewNeuronEscrowCaller creates a new read-only instance of NeuronEscrow, bound to a specific deployed contract.
func NewNeuronEscrowCaller(address common.Address, caller bind.ContractCaller) (*NeuronEscrowCaller, error) {
	contract, err := bindNeuronEscrow(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &NeuronEscrowCaller{contract: contract}, nil
}

// NewNeuronEscrowTransactor creates a new write-only instance of NeuronEscrow, bound to a specific deployed contract.
func NewNeuronEscrowTransactor(address common.Address, transactor bind.ContractTransactor) (*NeuronEscrowTransactor, error) {
	contract, err := bindNeuronEscrow(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &NeuronEscrowTransactor{contract: contract}, nil
}

// NewNeuronEscrowFilterer creates a new log filterer instance of NeuronEscrow, bound to a specific deployed contract.
func NewNeuronEscrowFilterer(address common.Address, filterer bind.ContractFilterer) (*NeuronEscrowFilterer, error) {
	contract, err := bindNeuronEscrow(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &NeuronEscrowFilterer{contract: contract}, nil
}

// bindNeuronEscrow binds a generic wrapper to an already deployed contract.
func bindNeuronEscrow(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := NeuronEscrowMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NeuronEscrow *NeuronEscrowRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NeuronEscrow.Contract.NeuronEscrowCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NeuronEscrow *NeuronEscrowRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.NeuronEscrowTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NeuronEscrow *NeuronEscrowRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.NeuronEscrowTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NeuronEscrow *NeuronEscrowCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NeuronEscrow.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NeuronEscrow *NeuronEscrowTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NeuronEscrow *NeuronEscrowTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.contract.Transact(opts, method, params...)
}

// GetBalance is a free data retrieval call binding the contract method 0x1e010439.
//
// Solidity: function getBalance(uint256 escrowId) view returns(uint256 available)
func (_NeuronEscrow *NeuronEscrowCaller) GetBalance(opts *bind.CallOpts, escrowId *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _NeuronEscrow.contract.Call(opts, &out, "getBalance", escrowId)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetBalance is a free data retrieval call binding the contract method 0x1e010439.
//
// Solidity: function getBalance(uint256 escrowId) view returns(uint256 available)
func (_NeuronEscrow *NeuronEscrowSession) GetBalance(escrowId *big.Int) (*big.Int, error) {
	return _NeuronEscrow.Contract.GetBalance(&_NeuronEscrow.CallOpts, escrowId)
}

// GetBalance is a free data retrieval call binding the contract method 0x1e010439.
//
// Solidity: function getBalance(uint256 escrowId) view returns(uint256 available)
func (_NeuronEscrow *NeuronEscrowCallerSession) GetBalance(escrowId *big.Int) (*big.Int, error) {
	return _NeuronEscrow.Contract.GetBalance(&_NeuronEscrow.CallOpts, escrowId)
}

// GetEscrow is a free data retrieval call binding the contract method 0x7d19e596.
//
// Solidity: function getEscrow(uint256 escrowId) view returns(address buyer, address seller, address arbiter, address token, uint64 threshold, bytes32 agreementHash, uint64 timeout, uint256 balance, uint256 pendingReleaseTotal, uint8 state)
func (_NeuronEscrow *NeuronEscrowCaller) GetEscrow(opts *bind.CallOpts, escrowId *big.Int) (struct {
	Buyer               common.Address
	Seller              common.Address
	Arbiter             common.Address
	Token               common.Address
	Threshold           uint64
	AgreementHash       [32]byte
	Timeout             uint64
	Balance             *big.Int
	PendingReleaseTotal *big.Int
	State               uint8
}, error) {
	var out []interface{}
	err := _NeuronEscrow.contract.Call(opts, &out, "getEscrow", escrowId)

	outstruct := new(struct {
		Buyer               common.Address
		Seller              common.Address
		Arbiter             common.Address
		Token               common.Address
		Threshold           uint64
		AgreementHash       [32]byte
		Timeout             uint64
		Balance             *big.Int
		PendingReleaseTotal *big.Int
		State               uint8
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Buyer = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.Seller = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.Arbiter = *abi.ConvertType(out[2], new(common.Address)).(*common.Address)
	outstruct.Token = *abi.ConvertType(out[3], new(common.Address)).(*common.Address)
	outstruct.Threshold = *abi.ConvertType(out[4], new(uint64)).(*uint64)
	outstruct.AgreementHash = *abi.ConvertType(out[5], new([32]byte)).(*[32]byte)
	outstruct.Timeout = *abi.ConvertType(out[6], new(uint64)).(*uint64)
	outstruct.Balance = *abi.ConvertType(out[7], new(*big.Int)).(**big.Int)
	outstruct.PendingReleaseTotal = *abi.ConvertType(out[8], new(*big.Int)).(**big.Int)
	outstruct.State = *abi.ConvertType(out[9], new(uint8)).(*uint8)

	return *outstruct, err

}

// GetEscrow is a free data retrieval call binding the contract method 0x7d19e596.
//
// Solidity: function getEscrow(uint256 escrowId) view returns(address buyer, address seller, address arbiter, address token, uint64 threshold, bytes32 agreementHash, uint64 timeout, uint256 balance, uint256 pendingReleaseTotal, uint8 state)
func (_NeuronEscrow *NeuronEscrowSession) GetEscrow(escrowId *big.Int) (struct {
	Buyer               common.Address
	Seller              common.Address
	Arbiter             common.Address
	Token               common.Address
	Threshold           uint64
	AgreementHash       [32]byte
	Timeout             uint64
	Balance             *big.Int
	PendingReleaseTotal *big.Int
	State               uint8
}, error) {
	return _NeuronEscrow.Contract.GetEscrow(&_NeuronEscrow.CallOpts, escrowId)
}

// GetEscrow is a free data retrieval call binding the contract method 0x7d19e596.
//
// Solidity: function getEscrow(uint256 escrowId) view returns(address buyer, address seller, address arbiter, address token, uint64 threshold, bytes32 agreementHash, uint64 timeout, uint256 balance, uint256 pendingReleaseTotal, uint8 state)
func (_NeuronEscrow *NeuronEscrowCallerSession) GetEscrow(escrowId *big.Int) (struct {
	Buyer               common.Address
	Seller              common.Address
	Arbiter             common.Address
	Token               common.Address
	Threshold           uint64
	AgreementHash       [32]byte
	Timeout             uint64
	Balance             *big.Int
	PendingReleaseTotal *big.Int
	State               uint8
}, error) {
	return _NeuronEscrow.Contract.GetEscrow(&_NeuronEscrow.CallOpts, escrowId)
}

// GetRelease is a free data retrieval call binding the contract method 0x1f4eb738.
//
// Solidity: function getRelease(uint256 escrowId, uint256 releaseId) view returns(uint256 amount, address recipient, bytes32 evidenceHash, uint8 state)
func (_NeuronEscrow *NeuronEscrowCaller) GetRelease(opts *bind.CallOpts, escrowId *big.Int, releaseId *big.Int) (struct {
	Amount       *big.Int
	Recipient    common.Address
	EvidenceHash [32]byte
	State        uint8
}, error) {
	var out []interface{}
	err := _NeuronEscrow.contract.Call(opts, &out, "getRelease", escrowId, releaseId)

	outstruct := new(struct {
		Amount       *big.Int
		Recipient    common.Address
		EvidenceHash [32]byte
		State        uint8
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Amount = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Recipient = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.EvidenceHash = *abi.ConvertType(out[2], new([32]byte)).(*[32]byte)
	outstruct.State = *abi.ConvertType(out[3], new(uint8)).(*uint8)

	return *outstruct, err

}

// GetRelease is a free data retrieval call binding the contract method 0x1f4eb738.
//
// Solidity: function getRelease(uint256 escrowId, uint256 releaseId) view returns(uint256 amount, address recipient, bytes32 evidenceHash, uint8 state)
func (_NeuronEscrow *NeuronEscrowSession) GetRelease(escrowId *big.Int, releaseId *big.Int) (struct {
	Amount       *big.Int
	Recipient    common.Address
	EvidenceHash [32]byte
	State        uint8
}, error) {
	return _NeuronEscrow.Contract.GetRelease(&_NeuronEscrow.CallOpts, escrowId, releaseId)
}

// GetRelease is a free data retrieval call binding the contract method 0x1f4eb738.
//
// Solidity: function getRelease(uint256 escrowId, uint256 releaseId) view returns(uint256 amount, address recipient, bytes32 evidenceHash, uint8 state)
func (_NeuronEscrow *NeuronEscrowCallerSession) GetRelease(escrowId *big.Int, releaseId *big.Int) (struct {
	Amount       *big.Int
	Recipient    common.Address
	EvidenceHash [32]byte
	State        uint8
}, error) {
	return _NeuronEscrow.Contract.GetRelease(&_NeuronEscrow.CallOpts, escrowId, releaseId)
}

// ApproveRelease is a paid mutator transaction binding the contract method 0xcb00801e.
//
// Solidity: function approveRelease(uint256 escrowId, uint256 releaseId) returns()
func (_NeuronEscrow *NeuronEscrowTransactor) ApproveRelease(opts *bind.TransactOpts, escrowId *big.Int, releaseId *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.contract.Transact(opts, "approveRelease", escrowId, releaseId)
}

// ApproveRelease is a paid mutator transaction binding the contract method 0xcb00801e.
//
// Solidity: function approveRelease(uint256 escrowId, uint256 releaseId) returns()
func (_NeuronEscrow *NeuronEscrowSession) ApproveRelease(escrowId *big.Int, releaseId *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.ApproveRelease(&_NeuronEscrow.TransactOpts, escrowId, releaseId)
}

// ApproveRelease is a paid mutator transaction binding the contract method 0xcb00801e.
//
// Solidity: function approveRelease(uint256 escrowId, uint256 releaseId) returns()
func (_NeuronEscrow *NeuronEscrowTransactorSession) ApproveRelease(escrowId *big.Int, releaseId *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.ApproveRelease(&_NeuronEscrow.TransactOpts, escrowId, releaseId)
}

// ClaimRefund is a paid mutator transaction binding the contract method 0x5b7baf64.
//
// Solidity: function claimRefund(uint256 escrowId) returns()
func (_NeuronEscrow *NeuronEscrowTransactor) ClaimRefund(opts *bind.TransactOpts, escrowId *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.contract.Transact(opts, "claimRefund", escrowId)
}

// ClaimRefund is a paid mutator transaction binding the contract method 0x5b7baf64.
//
// Solidity: function claimRefund(uint256 escrowId) returns()
func (_NeuronEscrow *NeuronEscrowSession) ClaimRefund(escrowId *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.ClaimRefund(&_NeuronEscrow.TransactOpts, escrowId)
}

// ClaimRefund is a paid mutator transaction binding the contract method 0x5b7baf64.
//
// Solidity: function claimRefund(uint256 escrowId) returns()
func (_NeuronEscrow *NeuronEscrowTransactorSession) ClaimRefund(escrowId *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.ClaimRefund(&_NeuronEscrow.TransactOpts, escrowId)
}

// CreateEscrow is a paid mutator transaction binding the contract method 0xa5f95add.
//
// Solidity: function createEscrow(address buyer, address seller, address arbiter, address token, uint64 threshold, bytes32 agreementHash, uint64 timeout) returns(uint256 escrowId)
func (_NeuronEscrow *NeuronEscrowTransactor) CreateEscrow(opts *bind.TransactOpts, buyer common.Address, seller common.Address, arbiter common.Address, token common.Address, threshold uint64, agreementHash [32]byte, timeout uint64) (*types.Transaction, error) {
	return _NeuronEscrow.contract.Transact(opts, "createEscrow", buyer, seller, arbiter, token, threshold, agreementHash, timeout)
}

// CreateEscrow is a paid mutator transaction binding the contract method 0xa5f95add.
//
// Solidity: function createEscrow(address buyer, address seller, address arbiter, address token, uint64 threshold, bytes32 agreementHash, uint64 timeout) returns(uint256 escrowId)
func (_NeuronEscrow *NeuronEscrowSession) CreateEscrow(buyer common.Address, seller common.Address, arbiter common.Address, token common.Address, threshold uint64, agreementHash [32]byte, timeout uint64) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.CreateEscrow(&_NeuronEscrow.TransactOpts, buyer, seller, arbiter, token, threshold, agreementHash, timeout)
}

// CreateEscrow is a paid mutator transaction binding the contract method 0xa5f95add.
//
// Solidity: function createEscrow(address buyer, address seller, address arbiter, address token, uint64 threshold, bytes32 agreementHash, uint64 timeout) returns(uint256 escrowId)
func (_NeuronEscrow *NeuronEscrowTransactorSession) CreateEscrow(buyer common.Address, seller common.Address, arbiter common.Address, token common.Address, threshold uint64, agreementHash [32]byte, timeout uint64) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.CreateEscrow(&_NeuronEscrow.TransactOpts, buyer, seller, arbiter, token, threshold, agreementHash, timeout)
}

// Deposit is a paid mutator transaction binding the contract method 0xe2bbb158.
//
// Solidity: function deposit(uint256 escrowId, uint256 amount) returns()
func (_NeuronEscrow *NeuronEscrowTransactor) Deposit(opts *bind.TransactOpts, escrowId *big.Int, amount *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.contract.Transact(opts, "deposit", escrowId, amount)
}

// Deposit is a paid mutator transaction binding the contract method 0xe2bbb158.
//
// Solidity: function deposit(uint256 escrowId, uint256 amount) returns()
func (_NeuronEscrow *NeuronEscrowSession) Deposit(escrowId *big.Int, amount *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.Deposit(&_NeuronEscrow.TransactOpts, escrowId, amount)
}

// Deposit is a paid mutator transaction binding the contract method 0xe2bbb158.
//
// Solidity: function deposit(uint256 escrowId, uint256 amount) returns()
func (_NeuronEscrow *NeuronEscrowTransactorSession) Deposit(escrowId *big.Int, amount *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.Deposit(&_NeuronEscrow.TransactOpts, escrowId, amount)
}

// RequestRelease is a paid mutator transaction binding the contract method 0x6e4a31db.
//
// Solidity: function requestRelease(uint256 escrowId, uint256 amount, address recipient, bytes32 evidenceHash) returns(uint256 releaseId)
func (_NeuronEscrow *NeuronEscrowTransactor) RequestRelease(opts *bind.TransactOpts, escrowId *big.Int, amount *big.Int, recipient common.Address, evidenceHash [32]byte) (*types.Transaction, error) {
	return _NeuronEscrow.contract.Transact(opts, "requestRelease", escrowId, amount, recipient, evidenceHash)
}

// RequestRelease is a paid mutator transaction binding the contract method 0x6e4a31db.
//
// Solidity: function requestRelease(uint256 escrowId, uint256 amount, address recipient, bytes32 evidenceHash) returns(uint256 releaseId)
func (_NeuronEscrow *NeuronEscrowSession) RequestRelease(escrowId *big.Int, amount *big.Int, recipient common.Address, evidenceHash [32]byte) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.RequestRelease(&_NeuronEscrow.TransactOpts, escrowId, amount, recipient, evidenceHash)
}

// RequestRelease is a paid mutator transaction binding the contract method 0x6e4a31db.
//
// Solidity: function requestRelease(uint256 escrowId, uint256 amount, address recipient, bytes32 evidenceHash) returns(uint256 releaseId)
func (_NeuronEscrow *NeuronEscrowTransactorSession) RequestRelease(escrowId *big.Int, amount *big.Int, recipient common.Address, evidenceHash [32]byte) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.RequestRelease(&_NeuronEscrow.TransactOpts, escrowId, amount, recipient, evidenceHash)
}

// Withdraw is a paid mutator transaction binding the contract method 0x441a3e70.
//
// Solidity: function withdraw(uint256 escrowId, uint256 releaseId) returns()
func (_NeuronEscrow *NeuronEscrowTransactor) Withdraw(opts *bind.TransactOpts, escrowId *big.Int, releaseId *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.contract.Transact(opts, "withdraw", escrowId, releaseId)
}

// Withdraw is a paid mutator transaction binding the contract method 0x441a3e70.
//
// Solidity: function withdraw(uint256 escrowId, uint256 releaseId) returns()
func (_NeuronEscrow *NeuronEscrowSession) Withdraw(escrowId *big.Int, releaseId *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.Withdraw(&_NeuronEscrow.TransactOpts, escrowId, releaseId)
}

// Withdraw is a paid mutator transaction binding the contract method 0x441a3e70.
//
// Solidity: function withdraw(uint256 escrowId, uint256 releaseId) returns()
func (_NeuronEscrow *NeuronEscrowTransactorSession) Withdraw(escrowId *big.Int, releaseId *big.Int) (*types.Transaction, error) {
	return _NeuronEscrow.Contract.Withdraw(&_NeuronEscrow.TransactOpts, escrowId, releaseId)
}

// NeuronEscrowDepositedIterator is returned from FilterDeposited and is used to iterate over the raw logs and unpacked data for Deposited events raised by the NeuronEscrow contract.
type NeuronEscrowDepositedIterator struct {
	Event *NeuronEscrowDeposited // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NeuronEscrowDepositedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronEscrowDeposited)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NeuronEscrowDeposited)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NeuronEscrowDepositedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronEscrowDepositedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronEscrowDeposited represents a Deposited event raised by the NeuronEscrow contract.
type NeuronEscrowDeposited struct {
	EscrowId   *big.Int
	Depositor  common.Address
	Amount     *big.Int
	NewBalance *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterDeposited is a free log retrieval operation binding the contract event 0xad5b4075b97dbf75ad5c78f7afac948e4ae611c4fdf2825e2ce3c6c96925bf3b.
//
// Solidity: event Deposited(uint256 indexed escrowId, address indexed depositor, uint256 amount, uint256 newBalance)
func (_NeuronEscrow *NeuronEscrowFilterer) FilterDeposited(opts *bind.FilterOpts, escrowId []*big.Int, depositor []common.Address) (*NeuronEscrowDepositedIterator, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var depositorRule []interface{}
	for _, depositorItem := range depositor {
		depositorRule = append(depositorRule, depositorItem)
	}

	logs, sub, err := _NeuronEscrow.contract.FilterLogs(opts, "Deposited", escrowIdRule, depositorRule)
	if err != nil {
		return nil, err
	}
	return &NeuronEscrowDepositedIterator{contract: _NeuronEscrow.contract, event: "Deposited", logs: logs, sub: sub}, nil
}

// WatchDeposited is a free log subscription operation binding the contract event 0xad5b4075b97dbf75ad5c78f7afac948e4ae611c4fdf2825e2ce3c6c96925bf3b.
//
// Solidity: event Deposited(uint256 indexed escrowId, address indexed depositor, uint256 amount, uint256 newBalance)
func (_NeuronEscrow *NeuronEscrowFilterer) WatchDeposited(opts *bind.WatchOpts, sink chan<- *NeuronEscrowDeposited, escrowId []*big.Int, depositor []common.Address) (event.Subscription, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var depositorRule []interface{}
	for _, depositorItem := range depositor {
		depositorRule = append(depositorRule, depositorItem)
	}

	logs, sub, err := _NeuronEscrow.contract.WatchLogs(opts, "Deposited", escrowIdRule, depositorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronEscrowDeposited)
				if err := _NeuronEscrow.contract.UnpackLog(event, "Deposited", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDeposited is a log parse operation binding the contract event 0xad5b4075b97dbf75ad5c78f7afac948e4ae611c4fdf2825e2ce3c6c96925bf3b.
//
// Solidity: event Deposited(uint256 indexed escrowId, address indexed depositor, uint256 amount, uint256 newBalance)
func (_NeuronEscrow *NeuronEscrowFilterer) ParseDeposited(log types.Log) (*NeuronEscrowDeposited, error) {
	event := new(NeuronEscrowDeposited)
	if err := _NeuronEscrow.contract.UnpackLog(event, "Deposited", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NeuronEscrowEscrowCreatedIterator is returned from FilterEscrowCreated and is used to iterate over the raw logs and unpacked data for EscrowCreated events raised by the NeuronEscrow contract.
type NeuronEscrowEscrowCreatedIterator struct {
	Event *NeuronEscrowEscrowCreated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NeuronEscrowEscrowCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronEscrowEscrowCreated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NeuronEscrowEscrowCreated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NeuronEscrowEscrowCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronEscrowEscrowCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronEscrowEscrowCreated represents a EscrowCreated event raised by the NeuronEscrow contract.
type NeuronEscrowEscrowCreated struct {
	EscrowId      *big.Int
	Buyer         common.Address
	Seller        common.Address
	Token         common.Address
	AgreementHash [32]byte
	Timeout       uint64
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterEscrowCreated is a free log retrieval operation binding the contract event 0xb82e1117edbb466f074004163623779e929f4600d80b8c467760e073b8c1e7b4.
//
// Solidity: event EscrowCreated(uint256 indexed escrowId, address indexed buyer, address indexed seller, address token, bytes32 agreementHash, uint64 timeout)
func (_NeuronEscrow *NeuronEscrowFilterer) FilterEscrowCreated(opts *bind.FilterOpts, escrowId []*big.Int, buyer []common.Address, seller []common.Address) (*NeuronEscrowEscrowCreatedIterator, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var buyerRule []interface{}
	for _, buyerItem := range buyer {
		buyerRule = append(buyerRule, buyerItem)
	}
	var sellerRule []interface{}
	for _, sellerItem := range seller {
		sellerRule = append(sellerRule, sellerItem)
	}

	logs, sub, err := _NeuronEscrow.contract.FilterLogs(opts, "EscrowCreated", escrowIdRule, buyerRule, sellerRule)
	if err != nil {
		return nil, err
	}
	return &NeuronEscrowEscrowCreatedIterator{contract: _NeuronEscrow.contract, event: "EscrowCreated", logs: logs, sub: sub}, nil
}

// WatchEscrowCreated is a free log subscription operation binding the contract event 0xb82e1117edbb466f074004163623779e929f4600d80b8c467760e073b8c1e7b4.
//
// Solidity: event EscrowCreated(uint256 indexed escrowId, address indexed buyer, address indexed seller, address token, bytes32 agreementHash, uint64 timeout)
func (_NeuronEscrow *NeuronEscrowFilterer) WatchEscrowCreated(opts *bind.WatchOpts, sink chan<- *NeuronEscrowEscrowCreated, escrowId []*big.Int, buyer []common.Address, seller []common.Address) (event.Subscription, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var buyerRule []interface{}
	for _, buyerItem := range buyer {
		buyerRule = append(buyerRule, buyerItem)
	}
	var sellerRule []interface{}
	for _, sellerItem := range seller {
		sellerRule = append(sellerRule, sellerItem)
	}

	logs, sub, err := _NeuronEscrow.contract.WatchLogs(opts, "EscrowCreated", escrowIdRule, buyerRule, sellerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronEscrowEscrowCreated)
				if err := _NeuronEscrow.contract.UnpackLog(event, "EscrowCreated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseEscrowCreated is a log parse operation binding the contract event 0xb82e1117edbb466f074004163623779e929f4600d80b8c467760e073b8c1e7b4.
//
// Solidity: event EscrowCreated(uint256 indexed escrowId, address indexed buyer, address indexed seller, address token, bytes32 agreementHash, uint64 timeout)
func (_NeuronEscrow *NeuronEscrowFilterer) ParseEscrowCreated(log types.Log) (*NeuronEscrowEscrowCreated, error) {
	event := new(NeuronEscrowEscrowCreated)
	if err := _NeuronEscrow.contract.UnpackLog(event, "EscrowCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NeuronEscrowRefundClaimedIterator is returned from FilterRefundClaimed and is used to iterate over the raw logs and unpacked data for RefundClaimed events raised by the NeuronEscrow contract.
type NeuronEscrowRefundClaimedIterator struct {
	Event *NeuronEscrowRefundClaimed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NeuronEscrowRefundClaimedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronEscrowRefundClaimed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NeuronEscrowRefundClaimed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NeuronEscrowRefundClaimedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronEscrowRefundClaimedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronEscrowRefundClaimed represents a RefundClaimed event raised by the NeuronEscrow contract.
type NeuronEscrowRefundClaimed struct {
	EscrowId *big.Int
	Buyer    common.Address
	Amount   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterRefundClaimed is a free log retrieval operation binding the contract event 0xf3f402280ef0a7905e124aa621b65eaeb2725c343e8b36d398ed78c29daf285c.
//
// Solidity: event RefundClaimed(uint256 indexed escrowId, address indexed buyer, uint256 amount)
func (_NeuronEscrow *NeuronEscrowFilterer) FilterRefundClaimed(opts *bind.FilterOpts, escrowId []*big.Int, buyer []common.Address) (*NeuronEscrowRefundClaimedIterator, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var buyerRule []interface{}
	for _, buyerItem := range buyer {
		buyerRule = append(buyerRule, buyerItem)
	}

	logs, sub, err := _NeuronEscrow.contract.FilterLogs(opts, "RefundClaimed", escrowIdRule, buyerRule)
	if err != nil {
		return nil, err
	}
	return &NeuronEscrowRefundClaimedIterator{contract: _NeuronEscrow.contract, event: "RefundClaimed", logs: logs, sub: sub}, nil
}

// WatchRefundClaimed is a free log subscription operation binding the contract event 0xf3f402280ef0a7905e124aa621b65eaeb2725c343e8b36d398ed78c29daf285c.
//
// Solidity: event RefundClaimed(uint256 indexed escrowId, address indexed buyer, uint256 amount)
func (_NeuronEscrow *NeuronEscrowFilterer) WatchRefundClaimed(opts *bind.WatchOpts, sink chan<- *NeuronEscrowRefundClaimed, escrowId []*big.Int, buyer []common.Address) (event.Subscription, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var buyerRule []interface{}
	for _, buyerItem := range buyer {
		buyerRule = append(buyerRule, buyerItem)
	}

	logs, sub, err := _NeuronEscrow.contract.WatchLogs(opts, "RefundClaimed", escrowIdRule, buyerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronEscrowRefundClaimed)
				if err := _NeuronEscrow.contract.UnpackLog(event, "RefundClaimed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRefundClaimed is a log parse operation binding the contract event 0xf3f402280ef0a7905e124aa621b65eaeb2725c343e8b36d398ed78c29daf285c.
//
// Solidity: event RefundClaimed(uint256 indexed escrowId, address indexed buyer, uint256 amount)
func (_NeuronEscrow *NeuronEscrowFilterer) ParseRefundClaimed(log types.Log) (*NeuronEscrowRefundClaimed, error) {
	event := new(NeuronEscrowRefundClaimed)
	if err := _NeuronEscrow.contract.UnpackLog(event, "RefundClaimed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NeuronEscrowReleaseApprovedIterator is returned from FilterReleaseApproved and is used to iterate over the raw logs and unpacked data for ReleaseApproved events raised by the NeuronEscrow contract.
type NeuronEscrowReleaseApprovedIterator struct {
	Event *NeuronEscrowReleaseApproved // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NeuronEscrowReleaseApprovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronEscrowReleaseApproved)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NeuronEscrowReleaseApproved)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NeuronEscrowReleaseApprovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronEscrowReleaseApprovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronEscrowReleaseApproved represents a ReleaseApproved event raised by the NeuronEscrow contract.
type NeuronEscrowReleaseApproved struct {
	EscrowId  *big.Int
	ReleaseId *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterReleaseApproved is a free log retrieval operation binding the contract event 0x1ad3f5d5c07752e6a836347b4cd670dbba93b657c0094d4a3d0faaaa3d5ebba8.
//
// Solidity: event ReleaseApproved(uint256 indexed escrowId, uint256 indexed releaseId)
func (_NeuronEscrow *NeuronEscrowFilterer) FilterReleaseApproved(opts *bind.FilterOpts, escrowId []*big.Int, releaseId []*big.Int) (*NeuronEscrowReleaseApprovedIterator, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var releaseIdRule []interface{}
	for _, releaseIdItem := range releaseId {
		releaseIdRule = append(releaseIdRule, releaseIdItem)
	}

	logs, sub, err := _NeuronEscrow.contract.FilterLogs(opts, "ReleaseApproved", escrowIdRule, releaseIdRule)
	if err != nil {
		return nil, err
	}
	return &NeuronEscrowReleaseApprovedIterator{contract: _NeuronEscrow.contract, event: "ReleaseApproved", logs: logs, sub: sub}, nil
}

// WatchReleaseApproved is a free log subscription operation binding the contract event 0x1ad3f5d5c07752e6a836347b4cd670dbba93b657c0094d4a3d0faaaa3d5ebba8.
//
// Solidity: event ReleaseApproved(uint256 indexed escrowId, uint256 indexed releaseId)
func (_NeuronEscrow *NeuronEscrowFilterer) WatchReleaseApproved(opts *bind.WatchOpts, sink chan<- *NeuronEscrowReleaseApproved, escrowId []*big.Int, releaseId []*big.Int) (event.Subscription, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var releaseIdRule []interface{}
	for _, releaseIdItem := range releaseId {
		releaseIdRule = append(releaseIdRule, releaseIdItem)
	}

	logs, sub, err := _NeuronEscrow.contract.WatchLogs(opts, "ReleaseApproved", escrowIdRule, releaseIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronEscrowReleaseApproved)
				if err := _NeuronEscrow.contract.UnpackLog(event, "ReleaseApproved", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseReleaseApproved is a log parse operation binding the contract event 0x1ad3f5d5c07752e6a836347b4cd670dbba93b657c0094d4a3d0faaaa3d5ebba8.
//
// Solidity: event ReleaseApproved(uint256 indexed escrowId, uint256 indexed releaseId)
func (_NeuronEscrow *NeuronEscrowFilterer) ParseReleaseApproved(log types.Log) (*NeuronEscrowReleaseApproved, error) {
	event := new(NeuronEscrowReleaseApproved)
	if err := _NeuronEscrow.contract.UnpackLog(event, "ReleaseApproved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NeuronEscrowReleaseRequestedIterator is returned from FilterReleaseRequested and is used to iterate over the raw logs and unpacked data for ReleaseRequested events raised by the NeuronEscrow contract.
type NeuronEscrowReleaseRequestedIterator struct {
	Event *NeuronEscrowReleaseRequested // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NeuronEscrowReleaseRequestedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronEscrowReleaseRequested)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NeuronEscrowReleaseRequested)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NeuronEscrowReleaseRequestedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronEscrowReleaseRequestedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronEscrowReleaseRequested represents a ReleaseRequested event raised by the NeuronEscrow contract.
type NeuronEscrowReleaseRequested struct {
	EscrowId     *big.Int
	ReleaseId    *big.Int
	Amount       *big.Int
	Recipient    common.Address
	EvidenceHash [32]byte
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterReleaseRequested is a free log retrieval operation binding the contract event 0xd4e5d6091fb42e83daa719b2a3466be650e5519653d8eb32312fa014c3703217.
//
// Solidity: event ReleaseRequested(uint256 indexed escrowId, uint256 indexed releaseId, uint256 amount, address recipient, bytes32 evidenceHash)
func (_NeuronEscrow *NeuronEscrowFilterer) FilterReleaseRequested(opts *bind.FilterOpts, escrowId []*big.Int, releaseId []*big.Int) (*NeuronEscrowReleaseRequestedIterator, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var releaseIdRule []interface{}
	for _, releaseIdItem := range releaseId {
		releaseIdRule = append(releaseIdRule, releaseIdItem)
	}

	logs, sub, err := _NeuronEscrow.contract.FilterLogs(opts, "ReleaseRequested", escrowIdRule, releaseIdRule)
	if err != nil {
		return nil, err
	}
	return &NeuronEscrowReleaseRequestedIterator{contract: _NeuronEscrow.contract, event: "ReleaseRequested", logs: logs, sub: sub}, nil
}

// WatchReleaseRequested is a free log subscription operation binding the contract event 0xd4e5d6091fb42e83daa719b2a3466be650e5519653d8eb32312fa014c3703217.
//
// Solidity: event ReleaseRequested(uint256 indexed escrowId, uint256 indexed releaseId, uint256 amount, address recipient, bytes32 evidenceHash)
func (_NeuronEscrow *NeuronEscrowFilterer) WatchReleaseRequested(opts *bind.WatchOpts, sink chan<- *NeuronEscrowReleaseRequested, escrowId []*big.Int, releaseId []*big.Int) (event.Subscription, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var releaseIdRule []interface{}
	for _, releaseIdItem := range releaseId {
		releaseIdRule = append(releaseIdRule, releaseIdItem)
	}

	logs, sub, err := _NeuronEscrow.contract.WatchLogs(opts, "ReleaseRequested", escrowIdRule, releaseIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronEscrowReleaseRequested)
				if err := _NeuronEscrow.contract.UnpackLog(event, "ReleaseRequested", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseReleaseRequested is a log parse operation binding the contract event 0xd4e5d6091fb42e83daa719b2a3466be650e5519653d8eb32312fa014c3703217.
//
// Solidity: event ReleaseRequested(uint256 indexed escrowId, uint256 indexed releaseId, uint256 amount, address recipient, bytes32 evidenceHash)
func (_NeuronEscrow *NeuronEscrowFilterer) ParseReleaseRequested(log types.Log) (*NeuronEscrowReleaseRequested, error) {
	event := new(NeuronEscrowReleaseRequested)
	if err := _NeuronEscrow.contract.UnpackLog(event, "ReleaseRequested", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NeuronEscrowWithdrawnIterator is returned from FilterWithdrawn and is used to iterate over the raw logs and unpacked data for Withdrawn events raised by the NeuronEscrow contract.
type NeuronEscrowWithdrawnIterator struct {
	Event *NeuronEscrowWithdrawn // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NeuronEscrowWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronEscrowWithdrawn)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NeuronEscrowWithdrawn)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NeuronEscrowWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronEscrowWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronEscrowWithdrawn represents a Withdrawn event raised by the NeuronEscrow contract.
type NeuronEscrowWithdrawn struct {
	EscrowId  *big.Int
	ReleaseId *big.Int
	Recipient common.Address
	Amount    *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterWithdrawn is a free log retrieval operation binding the contract event 0xef05a23f979cd8b846e8a62f76d15195e9a92e83a36901eb7eceaa476c69d25c.
//
// Solidity: event Withdrawn(uint256 indexed escrowId, uint256 indexed releaseId, address indexed recipient, uint256 amount)
func (_NeuronEscrow *NeuronEscrowFilterer) FilterWithdrawn(opts *bind.FilterOpts, escrowId []*big.Int, releaseId []*big.Int, recipient []common.Address) (*NeuronEscrowWithdrawnIterator, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var releaseIdRule []interface{}
	for _, releaseIdItem := range releaseId {
		releaseIdRule = append(releaseIdRule, releaseIdItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _NeuronEscrow.contract.FilterLogs(opts, "Withdrawn", escrowIdRule, releaseIdRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return &NeuronEscrowWithdrawnIterator{contract: _NeuronEscrow.contract, event: "Withdrawn", logs: logs, sub: sub}, nil
}

// WatchWithdrawn is a free log subscription operation binding the contract event 0xef05a23f979cd8b846e8a62f76d15195e9a92e83a36901eb7eceaa476c69d25c.
//
// Solidity: event Withdrawn(uint256 indexed escrowId, uint256 indexed releaseId, address indexed recipient, uint256 amount)
func (_NeuronEscrow *NeuronEscrowFilterer) WatchWithdrawn(opts *bind.WatchOpts, sink chan<- *NeuronEscrowWithdrawn, escrowId []*big.Int, releaseId []*big.Int, recipient []common.Address) (event.Subscription, error) {

	var escrowIdRule []interface{}
	for _, escrowIdItem := range escrowId {
		escrowIdRule = append(escrowIdRule, escrowIdItem)
	}
	var releaseIdRule []interface{}
	for _, releaseIdItem := range releaseId {
		releaseIdRule = append(releaseIdRule, releaseIdItem)
	}
	var recipientRule []interface{}
	for _, recipientItem := range recipient {
		recipientRule = append(recipientRule, recipientItem)
	}

	logs, sub, err := _NeuronEscrow.contract.WatchLogs(opts, "Withdrawn", escrowIdRule, releaseIdRule, recipientRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronEscrowWithdrawn)
				if err := _NeuronEscrow.contract.UnpackLog(event, "Withdrawn", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseWithdrawn is a log parse operation binding the contract event 0xef05a23f979cd8b846e8a62f76d15195e9a92e83a36901eb7eceaa476c69d25c.
//
// Solidity: event Withdrawn(uint256 indexed escrowId, uint256 indexed releaseId, address indexed recipient, uint256 amount)
func (_NeuronEscrow *NeuronEscrowFilterer) ParseWithdrawn(log types.Log) (*NeuronEscrowWithdrawn, error) {
	event := new(NeuronEscrowWithdrawn)
	if err := _NeuronEscrow.contract.UnpackLog(event, "Withdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
