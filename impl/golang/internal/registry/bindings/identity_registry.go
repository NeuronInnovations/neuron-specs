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

// NeuronIdentityRegistryMetaData contains all meta data concerning the NeuronIdentityRegistry contract.
var NeuronIdentityRegistryMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"agentURI\",\"inputs\":[{\"name\":\"agentId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"approve\",\"inputs\":[{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"balanceOf\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getApproved\",\"inputs\":[{\"name\":\"tokenId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isApprovedForAll\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"operator\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"lookup\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"agentId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"uri\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"name\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"owner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"ownerOf\",\"inputs\":[{\"name\":\"tokenId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"register\",\"inputs\":[{\"name\":\"_agentURI\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"agentId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"renounceOwnership\",\"inputs\":[],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"revoke\",\"inputs\":[{\"name\":\"agentId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"safeTransferFrom\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"safeTransferFrom\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setAgentURI\",\"inputs\":[{\"name\":\"agentId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"newURI\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setApprovalForAll\",\"inputs\":[{\"name\":\"operator\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"approved\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"supportsInterface\",\"inputs\":[{\"name\":\"interfaceId\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"symbol\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"tokenByIndex\",\"inputs\":[{\"name\":\"index\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"tokenOfOwnerByIndex\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"index\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"tokenURI\",\"inputs\":[{\"name\":\"agentId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"totalSupply\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"transferFrom\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"transferOwnership\",\"inputs\":[{\"name\":\"newOwner\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"Approval\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"approved\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"tokenId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ApprovalForAll\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"operator\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"approved\",\"type\":\"bool\",\"indexed\":false,\"internalType\":\"bool\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"IdentityRevoked\",\"inputs\":[{\"name\":\"agentId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"owner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"OwnershipTransferred\",\"inputs\":[{\"name\":\"previousOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"newOwner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Registered\",\"inputs\":[{\"name\":\"agentId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"agentURI\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"},{\"name\":\"owner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"Transfer\",\"inputs\":[{\"name\":\"from\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"to\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"tokenId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"URIUpdated\",\"inputs\":[{\"name\":\"agentId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"newURI\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"},{\"name\":\"updatedBy\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AlreadyRegistered\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC721EnumerableForbiddenBatchMint\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"ERC721IncorrectOwner\",\"inputs\":[{\"name\":\"sender\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC721InsufficientApproval\",\"inputs\":[{\"name\":\"operator\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"tokenId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"ERC721InvalidApprover\",\"inputs\":[{\"name\":\"approver\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC721InvalidOperator\",\"inputs\":[{\"name\":\"operator\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC721InvalidOwner\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC721InvalidReceiver\",\"inputs\":[{\"name\":\"receiver\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC721InvalidSender\",\"inputs\":[{\"name\":\"sender\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ERC721NonexistentToken\",\"inputs\":[{\"name\":\"tokenId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"ERC721OutOfBoundsIndex\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"index\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"EmptyAgentURI\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NotOwnerOrApproved\",\"inputs\":[{\"name\":\"agentId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"caller\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"NotTokenOwner\",\"inputs\":[{\"name\":\"agentId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"caller\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"OwnableInvalidOwner\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"OwnableUnauthorizedAccount\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ReentrancyGuardReentrantCall\",\"inputs\":[]}]",
	Bin: "0x60806040526001600d5534801562000015575f80fd5b50336040518060400160405280600f81526020016e4e6575726f6e204964656e7469747960881b8152506040518060400160405280600381526020016213925160ea1b815250815f90816200006b9190620001d0565b5060016200007a8282620001d0565b5050506001600160a01b038116620000ab57604051631e4fbdf760e01b81525f600482015260240160405180910390fd5b620000b681620000e1565b5060017f9b779b17422d0df92223018b32b4d1fa46e071723d6817e2486d003becc55f00556200029c565b600a80546001600160a01b038381166001600160a01b0319831681179093556040519116919082907f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0905f90a35050565b634e487b7160e01b5f52604160045260245ffd5b600181811c908216806200015b57607f821691505b6020821081036200017a57634e487b7160e01b5f52602260045260245ffd5b50919050565b601f821115620001cb57805f5260205f20601f840160051c81016020851015620001a75750805b601f840160051c820191505b81811015620001c8575f8155600101620001b3565b50505b505050565b81516001600160401b03811115620001ec57620001ec62000132565b6200020481620001fd845462000146565b8462000180565b602080601f8311600181146200023a575f8415620002225750858301515b5f19600386901b1c1916600185901b17855562000294565b5f85815260208120601f198616915b828110156200026a5788860151825594840194600190910190840162000249565b50858210156200028857878501515f19600388901b60f8161c191681555b505060018460011b0185555b505050505050565b611aaf80620002aa5f395ff3fe608060405234801561000f575f80fd5b5060043610610153575f3560e01c806370a08231116100bf578063b88d4fde11610079578063b88d4fde146102c5578063c87b56dd14610286578063d4b6b5da146102d8578063e985e9c5146102f9578063f2c298be1461030c578063f2fde38b1461031f575f80fd5b806370a082311461026b578063715018a61461027e57806378396cb3146102865780638da5cb5b1461029957806395d89b41146102aa578063a22cb465146102b2575f80fd5b806320c5429b1161011057806320c5429b146101f957806323b872dd1461020c5780632f745c591461021f57806342842e0e146102325780634f6ccce7146102455780636352211e14610258575f80fd5b806301ffc9a71461015757806306fdde031461017f578063081812fc14610194578063095ea7b3146101bf5780630af28bd3146101d457806318160ddd146101e7575b5f80fd5b61016a6101653660046114dd565b610332565b60405190151581526020015b60405180910390f35b61018761035c565b6040516101769190611542565b6101a76101a2366004611554565b6103eb565b6040516001600160a01b039091168152602001610176565b6101d26101cd366004611586565b610412565b005b6101d26101e23660046115f3565b610421565b6008545b604051908152602001610176565b6101d2610207366004611554565b6104f7565b6101d261021a36600461163b565b6105c5565b6101eb61022d366004611586565b61064e565b6101d261024036600461163b565b6106b1565b6101eb610253366004611554565b6106cb565b6101a7610266366004611554565b610720565b6101eb610279366004611674565b61072a565b6101d261076f565b610187610294366004611554565b610782565b600a546001600160a01b03166101a7565b610187610829565b6101d26102c036600461168d565b610838565b6101d26102d33660046116da565b610843565b6102eb6102e6366004611674565b61085b565b6040516101769291906117af565b61016a6103073660046117c7565b610917565b6101eb61031a3660046117f8565b610944565b6101d261032d366004611674565b610a38565b5f6001600160e01b0319821663780e9d6360e01b1480610356575061035682610a72565b92915050565b60605f805461036a90611837565b80601f016020809104026020016040519081016040528092919081815260200182805461039690611837565b80156103e15780601f106103b8576101008083540402835291602001916103e1565b820191905f5260205f20905b8154815290600101906020018083116103c457829003601f168201915b5050505050905090565b5f6103f582610ac1565b505f828152600460205260409020546001600160a01b0316610356565b61041d828233610af9565b5050565b610429610b06565b5f81900361044a5760405163657f26ff60e11b815260040160405180910390fd5b6104548333610b21565b61047f5760405163476b9b7560e01b8152600481018490523360248201526044015b60405180910390fd5b5f838152600c602052604090206104978284836118b3565b50336001600160a01b0316837f3a2c7fffc2cba7582c690e3b82c453ea02a308326a98a3ad7576c606336409fb84846040516104d492919061196d565b60405180910390a36104f260015f80516020611a5a83398151915255565b505050565b6104ff610b06565b5f61050982610720565b9050336001600160a01b0382161461053d57604051630da7a30b60e31b815260048101839052336024820152604401610476565b6001600160a01b0381165f908152600b60209081526040808320839055848352600c909152812061056d9161147e565b61057682610b7f565b6040516001600160a01b0382169083907f7b0104bb092ce7f934da29f0c0dcb9fa85e71475aca2777258facd66dca4dc6e905f90a3506105c260015f80516020611a5a83398151915255565b50565b6001600160a01b0382166105ee57604051633250574960e11b81525f6004820152602401610476565b5f6105fa838333610bb7565b9050836001600160a01b0316816001600160a01b031614610648576040516364283d7b60e01b81526001600160a01b0380861660048301526024820184905282166044820152606401610476565b50505050565b5f6106588361072a565b82106106895760405163295f44f760e21b81526001600160a01b038416600482015260248101839052604401610476565b506001600160a01b03919091165f908152600660209081526040808320938352929052205490565b6104f283838360405180602001604052805f815250610843565b5f6106d560085490565b82106106fd5760405163295f44f760e21b81525f600482015260248101839052604401610476565b600882815481106107105761071061199b565b905f5260205f2001549050919050565b5f61035682610ac1565b5f6001600160a01b038216610754576040516322718ad960e21b81525f6004820152602401610476565b506001600160a01b03165f9081526003602052604090205490565b610777610c59565b6107805f610c86565b565b606061078d82610ac1565b505f828152600c6020526040902080546107a690611837565b80601f01602080910402602001604051908101604052809291908181526020018280546107d290611837565b801561081d5780601f106107f45761010080835404028352916020019161081d565b820191905f5260205f20905b81548152906001019060200180831161080057829003601f168201915b50505050509050919050565b60606001805461036a90611837565b61041d338383610cd7565b61084e8484846105c5565b6106483385858585610d9e565b6001600160a01b0381165f908152600b602052604090205460608115610912575f828152600c60205260409020805461089390611837565b80601f01602080910402602001604051908101604052809291908181526020018280546108bf90611837565b801561090a5780601f106108e15761010080835404028352916020019161090a565b820191905f5260205f20905b8154815290600101906020018083116108ed57829003601f168201915b505050505090505b915091565b6001600160a01b039182165f90815260056020908152604080832093909416825291909152205460ff1690565b5f61094d610b06565b5f82900361096e5760405163657f26ff60e11b815260040160405180910390fd5b335f908152600b60205260409020541561099d576040516345ed80e960e01b8152336004820152602401610476565b600d8054905f6109ac836119c3565b9190505590506109bc3382610ec6565b5f818152600c602052604090206109d48385836118b3565b50335f818152600b6020526040908190208390555182907fca52e62c367d81bb2e328eb795f7c7ba24afb478408a26c0e201d155c449bc4a90610a1a908790879061196d565b60405180910390a361035660015f80516020611a5a83398151915255565b610a40610c59565b6001600160a01b038116610a6957604051631e4fbdf760e01b81525f6004820152602401610476565b6105c281610c86565b5f6001600160e01b031982166380ac58cd60e01b1480610aa257506001600160e01b03198216635b5e139f60e01b145b8061035657506301ffc9a760e01b6001600160e01b0319831614610356565b5f818152600260205260408120546001600160a01b03168061035657604051637e27328960e01b815260048101849052602401610476565b6104f28383836001610edf565b610b0e610fe3565b60025f80516020611a5a83398151915255565b5f80610b2c84610720565b9050806001600160a01b0316836001600160a01b03161480610b675750826001600160a01b0316610b5c856103eb565b6001600160a01b0316145b80610b775750610b778184610917565b949350505050565b5f610b8b5f835f610bb7565b90506001600160a01b03811661041d57604051637e27328960e01b815260048101839052602401610476565b5f80610bc4858585611012565b90506001600160a01b03811615801590610be657506001600160a01b03851615155b15610b77576001600160a01b0385165f908152600b602052604090205415610c2c576040516345ed80e960e01b81526001600160a01b0386166004820152602401610476565b6001600160a01b038082165f908152600b6020526040808220829055918716815220849055949350505050565b600a546001600160a01b031633146107805760405163118cdaa760e01b8152336004820152602401610476565b600a80546001600160a01b038381166001600160a01b0319831681179093556040519116919082907f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0905f90a35050565b6001600160a01b038316610d005760405163a9fbf51f60e01b81525f6004820152602401610476565b6001600160a01b038216610d3257604051630b61174360e31b81526001600160a01b0383166004820152602401610476565b6001600160a01b038381165f81815260056020908152604080832094871680845294825291829020805460ff191686151590811790915591519182527f17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31910160405180910390a3505050565b6001600160a01b0383163b15610ebf57604051630a85bd0160e11b81526001600160a01b0384169063150b7a0290610de09088908890879087906004016119db565b6020604051808303815f875af1925050508015610e1a575060408051601f3d908101601f19168201909252610e1791810190611a17565b60015b610e81573d808015610e47576040519150601f19603f3d011682016040523d82523d5f602084013e610e4c565b606091505b5080515f03610e7957604051633250574960e11b81526001600160a01b0385166004820152602401610476565b805160208201fd5b6001600160e01b03198116630a85bd0160e11b14610ebd57604051633250574960e11b81526001600160a01b0385166004820152602401610476565b505b5050505050565b61041d828260405180602001604052805f8152506110dd565b8080610ef357506001600160a01b03821615155b15610fb4575f610f0284610ac1565b90506001600160a01b03831615801590610f2e5750826001600160a01b0316816001600160a01b031614155b8015610f415750610f3f8184610917565b155b15610f6a5760405163a9fbf51f60e01b81526001600160a01b0384166004820152602401610476565b8115610fb25783856001600160a01b0316826001600160a01b03167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b92560405160405180910390a45b505b50505f90815260046020526040902080546001600160a01b0319166001600160a01b0392909216919091179055565b5f80516020611a5a8339815191525460020361078057604051633ee5aeb560e01b815260040160405180910390fd5b5f8061101f8585856110f4565b90506001600160a01b03811661107b5761107684600880545f838152600960205260408120829055600182018355919091527ff3f7a9fe364faab93b216da50a3214154f22a0a2b415b23a84c8169e8b636ee30155565b61109e565b846001600160a01b0316816001600160a01b03161461109e5761109e81856111e6565b6001600160a01b0385166110ba576110b584611263565b610b77565b846001600160a01b0316816001600160a01b031614610b7757610b77858561130a565b6110e78383611358565b6104f2335f858585610d9e565b5f828152600260205260408120546001600160a01b0390811690831615611120576111208184866113b9565b6001600160a01b0381161561115a5761113b5f855f80610edf565b6001600160a01b0381165f90815260036020526040902080545f190190555b6001600160a01b03851615611188576001600160a01b0385165f908152600360205260409020805460010190555b5f8481526002602052604080822080546001600160a01b0319166001600160a01b0389811691821790925591518793918516917fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef91a4949350505050565b5f6111f08361072a565b5f838152600760209081526040808320546001600160a01b0388168452600690925290912091925090818314611245575f83815260208281526040808320548584528184208190558352600790915290208290555b5f938452600760209081526040808620869055938552525081205550565b6008545f9061127490600190611a32565b5f838152600960205260408120546008805493945090928490811061129b5761129b61199b565b905f5260205f200154905080600883815481106112ba576112ba61199b565b5f9182526020808320909101929092558281526009909152604080822084905585825281205560088054806112f1576112f1611a45565b600190038181905f5260205f20015f9055905550505050565b5f60016113168461072a565b6113209190611a32565b6001600160a01b039093165f908152600660209081526040808320868452825280832085905593825260079052919091209190915550565b6001600160a01b03821661138157604051633250574960e11b81525f6004820152602401610476565b5f61138d83835f610bb7565b90506001600160a01b038116156104f2576040516339e3563760e11b81525f6004820152602401610476565b6113c483838361141d565b6104f2576001600160a01b0383166113f257604051637e27328960e01b815260048101829052602401610476565b60405163177e802f60e01b81526001600160a01b038316600482015260248101829052604401610476565b5f6001600160a01b03831615801590610b775750826001600160a01b0316846001600160a01b0316148061145657506114568484610917565b80610b775750505f908152600460205260409020546001600160a01b03908116911614919050565b50805461148a90611837565b5f825580601f10611499575050565b601f0160209004905f5260205f20908101906105c291905b808211156114c4575f81556001016114b1565b5090565b6001600160e01b0319811681146105c2575f80fd5b5f602082840312156114ed575f80fd5b81356114f8816114c8565b9392505050565b5f81518084525f5b8181101561152357602081850181015186830182015201611507565b505f602082860101526020601f19601f83011685010191505092915050565b602081525f6114f860208301846114ff565b5f60208284031215611564575f80fd5b5035919050565b80356001600160a01b0381168114611581575f80fd5b919050565b5f8060408385031215611597575f80fd5b6115a08361156b565b946020939093013593505050565b5f8083601f8401126115be575f80fd5b50813567ffffffffffffffff8111156115d5575f80fd5b6020830191508360208285010111156115ec575f80fd5b9250929050565b5f805f60408486031215611605575f80fd5b83359250602084013567ffffffffffffffff811115611622575f80fd5b61162e868287016115ae565b9497909650939450505050565b5f805f6060848603121561164d575f80fd5b6116568461156b565b92506116646020850161156b565b9150604084013590509250925092565b5f60208284031215611684575f80fd5b6114f88261156b565b5f806040838503121561169e575f80fd5b6116a78361156b565b9150602083013580151581146116bb575f80fd5b809150509250929050565b634e487b7160e01b5f52604160045260245ffd5b5f805f80608085870312156116ed575f80fd5b6116f68561156b565b93506117046020860161156b565b925060408501359150606085013567ffffffffffffffff80821115611727575f80fd5b818701915087601f83011261173a575f80fd5b81358181111561174c5761174c6116c6565b604051601f8201601f19908116603f01168101908382118183101715611774576117746116c6565b816040528281528a602084870101111561178c575f80fd5b826020860160208301375f60208483010152809550505050505092959194509250565b828152604060208201525f610b7760408301846114ff565b5f80604083850312156117d8575f80fd5b6117e18361156b565b91506117ef6020840161156b565b90509250929050565b5f8060208385031215611809575f80fd5b823567ffffffffffffffff81111561181f575f80fd5b61182b858286016115ae565b90969095509350505050565b600181811c9082168061184b57607f821691505b60208210810361186957634e487b7160e01b5f52602260045260245ffd5b50919050565b601f8211156104f257805f5260205f20601f840160051c810160208510156118945750805b601f840160051c820191505b81811015610ebf575f81556001016118a0565b67ffffffffffffffff8311156118cb576118cb6116c6565b6118df836118d98354611837565b8361186f565b5f601f841160018114611910575f85156118f95750838201355b5f19600387901b1c1916600186901b178355610ebf565b5f83815260208120601f198716915b8281101561193f578685013582556020948501946001909201910161191f565b508682101561195b575f1960f88860031b161c19848701351681555b505060018560011b0183555050505050565b60208152816020820152818360408301375f818301604090810191909152601f909201601f19160101919050565b634e487b7160e01b5f52603260045260245ffd5b634e487b7160e01b5f52601160045260245ffd5b5f600182016119d4576119d46119af565b5060010190565b6001600160a01b03858116825284166020820152604081018390526080606082018190525f90611a0d908301846114ff565b9695505050505050565b5f60208284031215611a27575f80fd5b81516114f8816114c8565b81810381811115610356576103566119af565b634e487b7160e01b5f52603160045260245ffdfe9b779b17422d0df92223018b32b4d1fa46e071723d6817e2486d003becc55f00a2646970667358221220d3dc2e5a9c708ab31909e3a2533bd8552fcaeaf207bd9c96b19571821ed3b2f464736f6c63430008180033",
}

// NeuronIdentityRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use NeuronIdentityRegistryMetaData.ABI instead.
var NeuronIdentityRegistryABI = NeuronIdentityRegistryMetaData.ABI

// NeuronIdentityRegistryBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use NeuronIdentityRegistryMetaData.Bin instead.
var NeuronIdentityRegistryBin = NeuronIdentityRegistryMetaData.Bin

// DeployNeuronIdentityRegistry deploys a new Ethereum contract, binding an instance of NeuronIdentityRegistry to it.
func DeployNeuronIdentityRegistry(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *NeuronIdentityRegistry, error) {
	parsed, err := NeuronIdentityRegistryMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(NeuronIdentityRegistryBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &NeuronIdentityRegistry{NeuronIdentityRegistryCaller: NeuronIdentityRegistryCaller{contract: contract}, NeuronIdentityRegistryTransactor: NeuronIdentityRegistryTransactor{contract: contract}, NeuronIdentityRegistryFilterer: NeuronIdentityRegistryFilterer{contract: contract}}, nil
}

// NeuronIdentityRegistry is an auto generated Go binding around an Ethereum contract.
type NeuronIdentityRegistry struct {
	NeuronIdentityRegistryCaller     // Read-only binding to the contract
	NeuronIdentityRegistryTransactor // Write-only binding to the contract
	NeuronIdentityRegistryFilterer   // Log filterer for contract events
}

// NeuronIdentityRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type NeuronIdentityRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NeuronIdentityRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type NeuronIdentityRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NeuronIdentityRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type NeuronIdentityRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NeuronIdentityRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type NeuronIdentityRegistrySession struct {
	Contract     *NeuronIdentityRegistry // Generic contract binding to set the session for
	CallOpts     bind.CallOpts           // Call options to use throughout this session
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// NeuronIdentityRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type NeuronIdentityRegistryCallerSession struct {
	Contract *NeuronIdentityRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                 // Call options to use throughout this session
}

// NeuronIdentityRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type NeuronIdentityRegistryTransactorSession struct {
	Contract     *NeuronIdentityRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                 // Transaction auth options to use throughout this session
}

// NeuronIdentityRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type NeuronIdentityRegistryRaw struct {
	Contract *NeuronIdentityRegistry // Generic contract binding to access the raw methods on
}

// NeuronIdentityRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type NeuronIdentityRegistryCallerRaw struct {
	Contract *NeuronIdentityRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// NeuronIdentityRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type NeuronIdentityRegistryTransactorRaw struct {
	Contract *NeuronIdentityRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewNeuronIdentityRegistry creates a new instance of NeuronIdentityRegistry, bound to a specific deployed contract.
func NewNeuronIdentityRegistry(address common.Address, backend bind.ContractBackend) (*NeuronIdentityRegistry, error) {
	contract, err := bindNeuronIdentityRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &NeuronIdentityRegistry{NeuronIdentityRegistryCaller: NeuronIdentityRegistryCaller{contract: contract}, NeuronIdentityRegistryTransactor: NeuronIdentityRegistryTransactor{contract: contract}, NeuronIdentityRegistryFilterer: NeuronIdentityRegistryFilterer{contract: contract}}, nil
}

// NewNeuronIdentityRegistryCaller creates a new read-only instance of NeuronIdentityRegistry, bound to a specific deployed contract.
func NewNeuronIdentityRegistryCaller(address common.Address, caller bind.ContractCaller) (*NeuronIdentityRegistryCaller, error) {
	contract, err := bindNeuronIdentityRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &NeuronIdentityRegistryCaller{contract: contract}, nil
}

// NewNeuronIdentityRegistryTransactor creates a new write-only instance of NeuronIdentityRegistry, bound to a specific deployed contract.
func NewNeuronIdentityRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*NeuronIdentityRegistryTransactor, error) {
	contract, err := bindNeuronIdentityRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &NeuronIdentityRegistryTransactor{contract: contract}, nil
}

// NewNeuronIdentityRegistryFilterer creates a new log filterer instance of NeuronIdentityRegistry, bound to a specific deployed contract.
func NewNeuronIdentityRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*NeuronIdentityRegistryFilterer, error) {
	contract, err := bindNeuronIdentityRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &NeuronIdentityRegistryFilterer{contract: contract}, nil
}

// bindNeuronIdentityRegistry binds a generic wrapper to an already deployed contract.
func bindNeuronIdentityRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := NeuronIdentityRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NeuronIdentityRegistry *NeuronIdentityRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NeuronIdentityRegistry.Contract.NeuronIdentityRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NeuronIdentityRegistry *NeuronIdentityRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.NeuronIdentityRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NeuronIdentityRegistry *NeuronIdentityRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.NeuronIdentityRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NeuronIdentityRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.contract.Transact(opts, method, params...)
}

// AgentURI is a free data retrieval call binding the contract method 0x78396cb3.
//
// Solidity: function agentURI(uint256 agentId) view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) AgentURI(opts *bind.CallOpts, agentId *big.Int) (string, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "agentURI", agentId)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// AgentURI is a free data retrieval call binding the contract method 0x78396cb3.
//
// Solidity: function agentURI(uint256 agentId) view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) AgentURI(agentId *big.Int) (string, error) {
	return _NeuronIdentityRegistry.Contract.AgentURI(&_NeuronIdentityRegistry.CallOpts, agentId)
}

// AgentURI is a free data retrieval call binding the contract method 0x78396cb3.
//
// Solidity: function agentURI(uint256 agentId) view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) AgentURI(agentId *big.Int) (string, error) {
	return _NeuronIdentityRegistry.Contract.AgentURI(&_NeuronIdentityRegistry.CallOpts, agentId)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) BalanceOf(opts *bind.CallOpts, owner common.Address) (*big.Int, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "balanceOf", owner)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) BalanceOf(owner common.Address) (*big.Int, error) {
	return _NeuronIdentityRegistry.Contract.BalanceOf(&_NeuronIdentityRegistry.CallOpts, owner)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address owner) view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) BalanceOf(owner common.Address) (*big.Int, error) {
	return _NeuronIdentityRegistry.Contract.BalanceOf(&_NeuronIdentityRegistry.CallOpts, owner)
}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) GetApproved(opts *bind.CallOpts, tokenId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "getApproved", tokenId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) GetApproved(tokenId *big.Int) (common.Address, error) {
	return _NeuronIdentityRegistry.Contract.GetApproved(&_NeuronIdentityRegistry.CallOpts, tokenId)
}

// GetApproved is a free data retrieval call binding the contract method 0x081812fc.
//
// Solidity: function getApproved(uint256 tokenId) view returns(address)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) GetApproved(tokenId *big.Int) (common.Address, error) {
	return _NeuronIdentityRegistry.Contract.GetApproved(&_NeuronIdentityRegistry.CallOpts, tokenId)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) IsApprovedForAll(opts *bind.CallOpts, owner common.Address, operator common.Address) (bool, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "isApprovedForAll", owner, operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) IsApprovedForAll(owner common.Address, operator common.Address) (bool, error) {
	return _NeuronIdentityRegistry.Contract.IsApprovedForAll(&_NeuronIdentityRegistry.CallOpts, owner, operator)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address owner, address operator) view returns(bool)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) IsApprovedForAll(owner common.Address, operator common.Address) (bool, error) {
	return _NeuronIdentityRegistry.Contract.IsApprovedForAll(&_NeuronIdentityRegistry.CallOpts, owner, operator)
}

// Lookup is a free data retrieval call binding the contract method 0xd4b6b5da.
//
// Solidity: function lookup(address account) view returns(uint256 agentId, string uri)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) Lookup(opts *bind.CallOpts, account common.Address) (struct {
	AgentId *big.Int
	Uri     string
}, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "lookup", account)

	outstruct := new(struct {
		AgentId *big.Int
		Uri     string
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.AgentId = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Uri = *abi.ConvertType(out[1], new(string)).(*string)

	return *outstruct, err

}

// Lookup is a free data retrieval call binding the contract method 0xd4b6b5da.
//
// Solidity: function lookup(address account) view returns(uint256 agentId, string uri)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) Lookup(account common.Address) (struct {
	AgentId *big.Int
	Uri     string
}, error) {
	return _NeuronIdentityRegistry.Contract.Lookup(&_NeuronIdentityRegistry.CallOpts, account)
}

// Lookup is a free data retrieval call binding the contract method 0xd4b6b5da.
//
// Solidity: function lookup(address account) view returns(uint256 agentId, string uri)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) Lookup(account common.Address) (struct {
	AgentId *big.Int
	Uri     string
}, error) {
	return _NeuronIdentityRegistry.Contract.Lookup(&_NeuronIdentityRegistry.CallOpts, account)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) Name() (string, error) {
	return _NeuronIdentityRegistry.Contract.Name(&_NeuronIdentityRegistry.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) Name() (string, error) {
	return _NeuronIdentityRegistry.Contract.Name(&_NeuronIdentityRegistry.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) Owner() (common.Address, error) {
	return _NeuronIdentityRegistry.Contract.Owner(&_NeuronIdentityRegistry.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) Owner() (common.Address, error) {
	return _NeuronIdentityRegistry.Contract.Owner(&_NeuronIdentityRegistry.CallOpts)
}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) OwnerOf(opts *bind.CallOpts, tokenId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "ownerOf", tokenId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) OwnerOf(tokenId *big.Int) (common.Address, error) {
	return _NeuronIdentityRegistry.Contract.OwnerOf(&_NeuronIdentityRegistry.CallOpts, tokenId)
}

// OwnerOf is a free data retrieval call binding the contract method 0x6352211e.
//
// Solidity: function ownerOf(uint256 tokenId) view returns(address)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) OwnerOf(tokenId *big.Int) (common.Address, error) {
	return _NeuronIdentityRegistry.Contract.OwnerOf(&_NeuronIdentityRegistry.CallOpts, tokenId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) SupportsInterface(opts *bind.CallOpts, interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "supportsInterface", interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _NeuronIdentityRegistry.Contract.SupportsInterface(&_NeuronIdentityRegistry.CallOpts, interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _NeuronIdentityRegistry.Contract.SupportsInterface(&_NeuronIdentityRegistry.CallOpts, interfaceId)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) Symbol() (string, error) {
	return _NeuronIdentityRegistry.Contract.Symbol(&_NeuronIdentityRegistry.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) Symbol() (string, error) {
	return _NeuronIdentityRegistry.Contract.Symbol(&_NeuronIdentityRegistry.CallOpts)
}

// TokenByIndex is a free data retrieval call binding the contract method 0x4f6ccce7.
//
// Solidity: function tokenByIndex(uint256 index) view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) TokenByIndex(opts *bind.CallOpts, index *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "tokenByIndex", index)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TokenByIndex is a free data retrieval call binding the contract method 0x4f6ccce7.
//
// Solidity: function tokenByIndex(uint256 index) view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) TokenByIndex(index *big.Int) (*big.Int, error) {
	return _NeuronIdentityRegistry.Contract.TokenByIndex(&_NeuronIdentityRegistry.CallOpts, index)
}

// TokenByIndex is a free data retrieval call binding the contract method 0x4f6ccce7.
//
// Solidity: function tokenByIndex(uint256 index) view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) TokenByIndex(index *big.Int) (*big.Int, error) {
	return _NeuronIdentityRegistry.Contract.TokenByIndex(&_NeuronIdentityRegistry.CallOpts, index)
}

// TokenOfOwnerByIndex is a free data retrieval call binding the contract method 0x2f745c59.
//
// Solidity: function tokenOfOwnerByIndex(address owner, uint256 index) view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) TokenOfOwnerByIndex(opts *bind.CallOpts, owner common.Address, index *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "tokenOfOwnerByIndex", owner, index)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TokenOfOwnerByIndex is a free data retrieval call binding the contract method 0x2f745c59.
//
// Solidity: function tokenOfOwnerByIndex(address owner, uint256 index) view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) TokenOfOwnerByIndex(owner common.Address, index *big.Int) (*big.Int, error) {
	return _NeuronIdentityRegistry.Contract.TokenOfOwnerByIndex(&_NeuronIdentityRegistry.CallOpts, owner, index)
}

// TokenOfOwnerByIndex is a free data retrieval call binding the contract method 0x2f745c59.
//
// Solidity: function tokenOfOwnerByIndex(address owner, uint256 index) view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) TokenOfOwnerByIndex(owner common.Address, index *big.Int) (*big.Int, error) {
	return _NeuronIdentityRegistry.Contract.TokenOfOwnerByIndex(&_NeuronIdentityRegistry.CallOpts, owner, index)
}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 agentId) view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) TokenURI(opts *bind.CallOpts, agentId *big.Int) (string, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "tokenURI", agentId)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 agentId) view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) TokenURI(agentId *big.Int) (string, error) {
	return _NeuronIdentityRegistry.Contract.TokenURI(&_NeuronIdentityRegistry.CallOpts, agentId)
}

// TokenURI is a free data retrieval call binding the contract method 0xc87b56dd.
//
// Solidity: function tokenURI(uint256 agentId) view returns(string)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) TokenURI(agentId *big.Int) (string, error) {
	return _NeuronIdentityRegistry.Contract.TokenURI(&_NeuronIdentityRegistry.CallOpts, agentId)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCaller) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _NeuronIdentityRegistry.contract.Call(opts, &out, "totalSupply")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) TotalSupply() (*big.Int, error) {
	return _NeuronIdentityRegistry.Contract.TotalSupply(&_NeuronIdentityRegistry.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryCallerSession) TotalSupply() (*big.Int, error) {
	return _NeuronIdentityRegistry.Contract.TotalSupply(&_NeuronIdentityRegistry.CallOpts)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactor) Approve(opts *bind.TransactOpts, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.contract.Transact(opts, "approve", to, tokenId)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) Approve(to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.Approve(&_NeuronIdentityRegistry.TransactOpts, to, tokenId)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address to, uint256 tokenId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorSession) Approve(to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.Approve(&_NeuronIdentityRegistry.TransactOpts, to, tokenId)
}

// Register is a paid mutator transaction binding the contract method 0xf2c298be.
//
// Solidity: function register(string _agentURI) returns(uint256 agentId)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactor) Register(opts *bind.TransactOpts, _agentURI string) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.contract.Transact(opts, "register", _agentURI)
}

// Register is a paid mutator transaction binding the contract method 0xf2c298be.
//
// Solidity: function register(string _agentURI) returns(uint256 agentId)
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) Register(_agentURI string) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.Register(&_NeuronIdentityRegistry.TransactOpts, _agentURI)
}

// Register is a paid mutator transaction binding the contract method 0xf2c298be.
//
// Solidity: function register(string _agentURI) returns(uint256 agentId)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorSession) Register(_agentURI string) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.Register(&_NeuronIdentityRegistry.TransactOpts, _agentURI)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) RenounceOwnership() (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.RenounceOwnership(&_NeuronIdentityRegistry.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.RenounceOwnership(&_NeuronIdentityRegistry.TransactOpts)
}

// Revoke is a paid mutator transaction binding the contract method 0x20c5429b.
//
// Solidity: function revoke(uint256 agentId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactor) Revoke(opts *bind.TransactOpts, agentId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.contract.Transact(opts, "revoke", agentId)
}

// Revoke is a paid mutator transaction binding the contract method 0x20c5429b.
//
// Solidity: function revoke(uint256 agentId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) Revoke(agentId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.Revoke(&_NeuronIdentityRegistry.TransactOpts, agentId)
}

// Revoke is a paid mutator transaction binding the contract method 0x20c5429b.
//
// Solidity: function revoke(uint256 agentId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorSession) Revoke(agentId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.Revoke(&_NeuronIdentityRegistry.TransactOpts, agentId)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactor) SafeTransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.contract.Transact(opts, "safeTransferFrom", from, to, tokenId)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) SafeTransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.SafeTransferFrom(&_NeuronIdentityRegistry.TransactOpts, from, to, tokenId)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0x42842e0e.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorSession) SafeTransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.SafeTransferFrom(&_NeuronIdentityRegistry.TransactOpts, from, to, tokenId)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactor) SafeTransferFrom0(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.contract.Transact(opts, "safeTransferFrom0", from, to, tokenId, data)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) SafeTransferFrom0(from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.SafeTransferFrom0(&_NeuronIdentityRegistry.TransactOpts, from, to, tokenId, data)
}

// SafeTransferFrom0 is a paid mutator transaction binding the contract method 0xb88d4fde.
//
// Solidity: function safeTransferFrom(address from, address to, uint256 tokenId, bytes data) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorSession) SafeTransferFrom0(from common.Address, to common.Address, tokenId *big.Int, data []byte) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.SafeTransferFrom0(&_NeuronIdentityRegistry.TransactOpts, from, to, tokenId, data)
}

// SetAgentURI is a paid mutator transaction binding the contract method 0x0af28bd3.
//
// Solidity: function setAgentURI(uint256 agentId, string newURI) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactor) SetAgentURI(opts *bind.TransactOpts, agentId *big.Int, newURI string) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.contract.Transact(opts, "setAgentURI", agentId, newURI)
}

// SetAgentURI is a paid mutator transaction binding the contract method 0x0af28bd3.
//
// Solidity: function setAgentURI(uint256 agentId, string newURI) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) SetAgentURI(agentId *big.Int, newURI string) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.SetAgentURI(&_NeuronIdentityRegistry.TransactOpts, agentId, newURI)
}

// SetAgentURI is a paid mutator transaction binding the contract method 0x0af28bd3.
//
// Solidity: function setAgentURI(uint256 agentId, string newURI) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorSession) SetAgentURI(agentId *big.Int, newURI string) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.SetAgentURI(&_NeuronIdentityRegistry.TransactOpts, agentId, newURI)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactor) SetApprovalForAll(opts *bind.TransactOpts, operator common.Address, approved bool) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.contract.Transact(opts, "setApprovalForAll", operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.SetApprovalForAll(&_NeuronIdentityRegistry.TransactOpts, operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorSession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.SetApprovalForAll(&_NeuronIdentityRegistry.TransactOpts, operator, approved)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactor) TransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.contract.Transact(opts, "transferFrom", from, to, tokenId)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) TransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.TransferFrom(&_NeuronIdentityRegistry.TransactOpts, from, to, tokenId)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 tokenId) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorSession) TransferFrom(from common.Address, to common.Address, tokenId *big.Int) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.TransferFrom(&_NeuronIdentityRegistry.TransactOpts, from, to, tokenId)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistrySession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.TransferOwnership(&_NeuronIdentityRegistry.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_NeuronIdentityRegistry *NeuronIdentityRegistryTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _NeuronIdentityRegistry.Contract.TransferOwnership(&_NeuronIdentityRegistry.TransactOpts, newOwner)
}

// NeuronIdentityRegistryApprovalIterator is returned from FilterApproval and is used to iterate over the raw logs and unpacked data for Approval events raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryApprovalIterator struct {
	Event *NeuronIdentityRegistryApproval // Event containing the contract specifics and raw log

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
func (it *NeuronIdentityRegistryApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronIdentityRegistryApproval)
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
		it.Event = new(NeuronIdentityRegistryApproval)
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
func (it *NeuronIdentityRegistryApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronIdentityRegistryApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronIdentityRegistryApproval represents a Approval event raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryApproval struct {
	Owner    common.Address
	Approved common.Address
	TokenId  *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterApproval is a free log retrieval operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) FilterApproval(opts *bind.FilterOpts, owner []common.Address, approved []common.Address, tokenId []*big.Int) (*NeuronIdentityRegistryApprovalIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var approvedRule []interface{}
	for _, approvedItem := range approved {
		approvedRule = append(approvedRule, approvedItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.FilterLogs(opts, "Approval", ownerRule, approvedRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &NeuronIdentityRegistryApprovalIterator{contract: _NeuronIdentityRegistry.contract, event: "Approval", logs: logs, sub: sub}, nil
}

// WatchApproval is a free log subscription operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) WatchApproval(opts *bind.WatchOpts, sink chan<- *NeuronIdentityRegistryApproval, owner []common.Address, approved []common.Address, tokenId []*big.Int) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var approvedRule []interface{}
	for _, approvedItem := range approved {
		approvedRule = append(approvedRule, approvedItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.WatchLogs(opts, "Approval", ownerRule, approvedRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronIdentityRegistryApproval)
				if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "Approval", log); err != nil {
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

// ParseApproval is a log parse operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) ParseApproval(log types.Log) (*NeuronIdentityRegistryApproval, error) {
	event := new(NeuronIdentityRegistryApproval)
	if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "Approval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NeuronIdentityRegistryApprovalForAllIterator is returned from FilterApprovalForAll and is used to iterate over the raw logs and unpacked data for ApprovalForAll events raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryApprovalForAllIterator struct {
	Event *NeuronIdentityRegistryApprovalForAll // Event containing the contract specifics and raw log

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
func (it *NeuronIdentityRegistryApprovalForAllIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronIdentityRegistryApprovalForAll)
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
		it.Event = new(NeuronIdentityRegistryApprovalForAll)
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
func (it *NeuronIdentityRegistryApprovalForAllIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronIdentityRegistryApprovalForAllIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronIdentityRegistryApprovalForAll represents a ApprovalForAll event raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryApprovalForAll struct {
	Owner    common.Address
	Operator common.Address
	Approved bool
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterApprovalForAll is a free log retrieval operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) FilterApprovalForAll(opts *bind.FilterOpts, owner []common.Address, operator []common.Address) (*NeuronIdentityRegistryApprovalForAllIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.FilterLogs(opts, "ApprovalForAll", ownerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &NeuronIdentityRegistryApprovalForAllIterator{contract: _NeuronIdentityRegistry.contract, event: "ApprovalForAll", logs: logs, sub: sub}, nil
}

// WatchApprovalForAll is a free log subscription operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) WatchApprovalForAll(opts *bind.WatchOpts, sink chan<- *NeuronIdentityRegistryApprovalForAll, owner []common.Address, operator []common.Address) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.WatchLogs(opts, "ApprovalForAll", ownerRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronIdentityRegistryApprovalForAll)
				if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
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

// ParseApprovalForAll is a log parse operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed owner, address indexed operator, bool approved)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) ParseApprovalForAll(log types.Log) (*NeuronIdentityRegistryApprovalForAll, error) {
	event := new(NeuronIdentityRegistryApprovalForAll)
	if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NeuronIdentityRegistryIdentityRevokedIterator is returned from FilterIdentityRevoked and is used to iterate over the raw logs and unpacked data for IdentityRevoked events raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryIdentityRevokedIterator struct {
	Event *NeuronIdentityRegistryIdentityRevoked // Event containing the contract specifics and raw log

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
func (it *NeuronIdentityRegistryIdentityRevokedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronIdentityRegistryIdentityRevoked)
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
		it.Event = new(NeuronIdentityRegistryIdentityRevoked)
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
func (it *NeuronIdentityRegistryIdentityRevokedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronIdentityRegistryIdentityRevokedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronIdentityRegistryIdentityRevoked represents a IdentityRevoked event raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryIdentityRevoked struct {
	AgentId *big.Int
	Owner   common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterIdentityRevoked is a free log retrieval operation binding the contract event 0x7b0104bb092ce7f934da29f0c0dcb9fa85e71475aca2777258facd66dca4dc6e.
//
// Solidity: event IdentityRevoked(uint256 indexed agentId, address indexed owner)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) FilterIdentityRevoked(opts *bind.FilterOpts, agentId []*big.Int, owner []common.Address) (*NeuronIdentityRegistryIdentityRevokedIterator, error) {

	var agentIdRule []interface{}
	for _, agentIdItem := range agentId {
		agentIdRule = append(agentIdRule, agentIdItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.FilterLogs(opts, "IdentityRevoked", agentIdRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return &NeuronIdentityRegistryIdentityRevokedIterator{contract: _NeuronIdentityRegistry.contract, event: "IdentityRevoked", logs: logs, sub: sub}, nil
}

// WatchIdentityRevoked is a free log subscription operation binding the contract event 0x7b0104bb092ce7f934da29f0c0dcb9fa85e71475aca2777258facd66dca4dc6e.
//
// Solidity: event IdentityRevoked(uint256 indexed agentId, address indexed owner)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) WatchIdentityRevoked(opts *bind.WatchOpts, sink chan<- *NeuronIdentityRegistryIdentityRevoked, agentId []*big.Int, owner []common.Address) (event.Subscription, error) {

	var agentIdRule []interface{}
	for _, agentIdItem := range agentId {
		agentIdRule = append(agentIdRule, agentIdItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.WatchLogs(opts, "IdentityRevoked", agentIdRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronIdentityRegistryIdentityRevoked)
				if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "IdentityRevoked", log); err != nil {
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

// ParseIdentityRevoked is a log parse operation binding the contract event 0x7b0104bb092ce7f934da29f0c0dcb9fa85e71475aca2777258facd66dca4dc6e.
//
// Solidity: event IdentityRevoked(uint256 indexed agentId, address indexed owner)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) ParseIdentityRevoked(log types.Log) (*NeuronIdentityRegistryIdentityRevoked, error) {
	event := new(NeuronIdentityRegistryIdentityRevoked)
	if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "IdentityRevoked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NeuronIdentityRegistryOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryOwnershipTransferredIterator struct {
	Event *NeuronIdentityRegistryOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *NeuronIdentityRegistryOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronIdentityRegistryOwnershipTransferred)
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
		it.Event = new(NeuronIdentityRegistryOwnershipTransferred)
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
func (it *NeuronIdentityRegistryOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronIdentityRegistryOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronIdentityRegistryOwnershipTransferred represents a OwnershipTransferred event raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*NeuronIdentityRegistryOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &NeuronIdentityRegistryOwnershipTransferredIterator{contract: _NeuronIdentityRegistry.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *NeuronIdentityRegistryOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronIdentityRegistryOwnershipTransferred)
				if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) ParseOwnershipTransferred(log types.Log) (*NeuronIdentityRegistryOwnershipTransferred, error) {
	event := new(NeuronIdentityRegistryOwnershipTransferred)
	if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NeuronIdentityRegistryRegisteredIterator is returned from FilterRegistered and is used to iterate over the raw logs and unpacked data for Registered events raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryRegisteredIterator struct {
	Event *NeuronIdentityRegistryRegistered // Event containing the contract specifics and raw log

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
func (it *NeuronIdentityRegistryRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronIdentityRegistryRegistered)
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
		it.Event = new(NeuronIdentityRegistryRegistered)
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
func (it *NeuronIdentityRegistryRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronIdentityRegistryRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronIdentityRegistryRegistered represents a Registered event raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryRegistered struct {
	AgentId  *big.Int
	AgentURI string
	Owner    common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterRegistered is a free log retrieval operation binding the contract event 0xca52e62c367d81bb2e328eb795f7c7ba24afb478408a26c0e201d155c449bc4a.
//
// Solidity: event Registered(uint256 indexed agentId, string agentURI, address indexed owner)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) FilterRegistered(opts *bind.FilterOpts, agentId []*big.Int, owner []common.Address) (*NeuronIdentityRegistryRegisteredIterator, error) {

	var agentIdRule []interface{}
	for _, agentIdItem := range agentId {
		agentIdRule = append(agentIdRule, agentIdItem)
	}

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.FilterLogs(opts, "Registered", agentIdRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return &NeuronIdentityRegistryRegisteredIterator{contract: _NeuronIdentityRegistry.contract, event: "Registered", logs: logs, sub: sub}, nil
}

// WatchRegistered is a free log subscription operation binding the contract event 0xca52e62c367d81bb2e328eb795f7c7ba24afb478408a26c0e201d155c449bc4a.
//
// Solidity: event Registered(uint256 indexed agentId, string agentURI, address indexed owner)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) WatchRegistered(opts *bind.WatchOpts, sink chan<- *NeuronIdentityRegistryRegistered, agentId []*big.Int, owner []common.Address) (event.Subscription, error) {

	var agentIdRule []interface{}
	for _, agentIdItem := range agentId {
		agentIdRule = append(agentIdRule, agentIdItem)
	}

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.WatchLogs(opts, "Registered", agentIdRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronIdentityRegistryRegistered)
				if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "Registered", log); err != nil {
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

// ParseRegistered is a log parse operation binding the contract event 0xca52e62c367d81bb2e328eb795f7c7ba24afb478408a26c0e201d155c449bc4a.
//
// Solidity: event Registered(uint256 indexed agentId, string agentURI, address indexed owner)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) ParseRegistered(log types.Log) (*NeuronIdentityRegistryRegistered, error) {
	event := new(NeuronIdentityRegistryRegistered)
	if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "Registered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NeuronIdentityRegistryTransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryTransferIterator struct {
	Event *NeuronIdentityRegistryTransfer // Event containing the contract specifics and raw log

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
func (it *NeuronIdentityRegistryTransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronIdentityRegistryTransfer)
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
		it.Event = new(NeuronIdentityRegistryTransfer)
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
func (it *NeuronIdentityRegistryTransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronIdentityRegistryTransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronIdentityRegistryTransfer represents a Transfer event raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryTransfer struct {
	From    common.Address
	To      common.Address
	TokenId *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) FilterTransfer(opts *bind.FilterOpts, from []common.Address, to []common.Address, tokenId []*big.Int) (*NeuronIdentityRegistryTransferIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.FilterLogs(opts, "Transfer", fromRule, toRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return &NeuronIdentityRegistryTransferIterator{contract: _NeuronIdentityRegistry.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *NeuronIdentityRegistryTransfer, from []common.Address, to []common.Address, tokenId []*big.Int) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}
	var tokenIdRule []interface{}
	for _, tokenIdItem := range tokenId {
		tokenIdRule = append(tokenIdRule, tokenIdItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.WatchLogs(opts, "Transfer", fromRule, toRule, tokenIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronIdentityRegistryTransfer)
				if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "Transfer", log); err != nil {
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

// ParseTransfer is a log parse operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) ParseTransfer(log types.Log) (*NeuronIdentityRegistryTransfer, error) {
	event := new(NeuronIdentityRegistryTransfer)
	if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NeuronIdentityRegistryURIUpdatedIterator is returned from FilterURIUpdated and is used to iterate over the raw logs and unpacked data for URIUpdated events raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryURIUpdatedIterator struct {
	Event *NeuronIdentityRegistryURIUpdated // Event containing the contract specifics and raw log

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
func (it *NeuronIdentityRegistryURIUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NeuronIdentityRegistryURIUpdated)
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
		it.Event = new(NeuronIdentityRegistryURIUpdated)
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
func (it *NeuronIdentityRegistryURIUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NeuronIdentityRegistryURIUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NeuronIdentityRegistryURIUpdated represents a URIUpdated event raised by the NeuronIdentityRegistry contract.
type NeuronIdentityRegistryURIUpdated struct {
	AgentId   *big.Int
	NewURI    string
	UpdatedBy common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterURIUpdated is a free log retrieval operation binding the contract event 0x3a2c7fffc2cba7582c690e3b82c453ea02a308326a98a3ad7576c606336409fb.
//
// Solidity: event URIUpdated(uint256 indexed agentId, string newURI, address indexed updatedBy)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) FilterURIUpdated(opts *bind.FilterOpts, agentId []*big.Int, updatedBy []common.Address) (*NeuronIdentityRegistryURIUpdatedIterator, error) {

	var agentIdRule []interface{}
	for _, agentIdItem := range agentId {
		agentIdRule = append(agentIdRule, agentIdItem)
	}

	var updatedByRule []interface{}
	for _, updatedByItem := range updatedBy {
		updatedByRule = append(updatedByRule, updatedByItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.FilterLogs(opts, "URIUpdated", agentIdRule, updatedByRule)
	if err != nil {
		return nil, err
	}
	return &NeuronIdentityRegistryURIUpdatedIterator{contract: _NeuronIdentityRegistry.contract, event: "URIUpdated", logs: logs, sub: sub}, nil
}

// WatchURIUpdated is a free log subscription operation binding the contract event 0x3a2c7fffc2cba7582c690e3b82c453ea02a308326a98a3ad7576c606336409fb.
//
// Solidity: event URIUpdated(uint256 indexed agentId, string newURI, address indexed updatedBy)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) WatchURIUpdated(opts *bind.WatchOpts, sink chan<- *NeuronIdentityRegistryURIUpdated, agentId []*big.Int, updatedBy []common.Address) (event.Subscription, error) {

	var agentIdRule []interface{}
	for _, agentIdItem := range agentId {
		agentIdRule = append(agentIdRule, agentIdItem)
	}

	var updatedByRule []interface{}
	for _, updatedByItem := range updatedBy {
		updatedByRule = append(updatedByRule, updatedByItem)
	}

	logs, sub, err := _NeuronIdentityRegistry.contract.WatchLogs(opts, "URIUpdated", agentIdRule, updatedByRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NeuronIdentityRegistryURIUpdated)
				if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "URIUpdated", log); err != nil {
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

// ParseURIUpdated is a log parse operation binding the contract event 0x3a2c7fffc2cba7582c690e3b82c453ea02a308326a98a3ad7576c606336409fb.
//
// Solidity: event URIUpdated(uint256 indexed agentId, string newURI, address indexed updatedBy)
func (_NeuronIdentityRegistry *NeuronIdentityRegistryFilterer) ParseURIUpdated(log types.Log) (*NeuronIdentityRegistryURIUpdated, error) {
	event := new(NeuronIdentityRegistryURIUpdated)
	if err := _NeuronIdentityRegistry.contract.UnpackLog(event, "URIUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
