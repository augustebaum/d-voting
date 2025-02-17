package evoting

import (
	"github.com/dedis/d-voting/services/dkg"
	"go.dedis.ch/dela/core/access"
	"go.dedis.ch/dela/core/execution"
	"go.dedis.ch/dela/core/execution/native"
	"go.dedis.ch/dela/core/store"

	"go.dedis.ch/kyber/v3/proof"
	"go.dedis.ch/kyber/v3/suites"
	"golang.org/x/xerrors"
)

const protocolName = "PairShuffle"
const messageOnlyOneShufflePerRound = "shuffle is already happening in this round"
const ElectionsMetadataKey = "ElectionsMetadataKey"

const errDecodeElectionID = "failed to decode Election ID: %v"
const errArgNotFound = "%q not found in tx arg"

var suite = suites.MustFind("Ed25519")

// TODO : the smart contract should create its own dkg Actor

const (
	// ContractName is the name of the contract.
	ContractName = "go.dedis.ch/dela.Evoting"

	// CmdArg is the argument's name to indicate the kind of command we want to
	// run on the contract. Should be one of the Command type.
	CmdArg = "evoting:command"

	CreateElectionArg = "evoting:create_election"

	CastVoteArg = "evoting:cast_vote"

	CancelElectionArg = "evoting:cancel_election"

	CloseElectionArg = "evoting:close_election"

	ShuffleBallotsArg = "evoting:shuffle_ballots"

	DecryptBallotsArg = "evoting:decrypt_ballots"

	// credentialAllCommand defines the credential command that is allowed to
	// perform all commands.
	credentialAllCommand = "all"
)

// commands defines the commands of the evoting contract.
type commands interface {
	createElection(snap store.Snapshot, step execution.Step, dkgActor dkg.Actor) error
	castVote(snap store.Snapshot, step execution.Step) error
	closeElection(snap store.Snapshot, step execution.Step) error
	shuffleBallots(snap store.Snapshot, step execution.Step) error
	decryptBallots(snap store.Snapshot, step execution.Step) error
	cancelElection(snap store.Snapshot, step execution.Step) error
}

// Command defines a type of command for the value contract
type Command string

const (
	CmdCreateElection Command = "CREATE_ELECTION"

	CmdCastVote Command = "CAST_VOTE"

	CmdCloseElection Command = "CLOSE_ELECTION"

	CmdShuffleBallots Command = "SHUFFLE_BALLOTS"

	CmdDecryptBallots Command = "DECRYPT_BALLOTS"

	CmdCancelElection Command = "CANCEL_ELECTION"
)

// NewCreds creates new credentials for a evoting contract execution. We might
// want to use in the future a separate credential for each command.
func NewCreds(id []byte) access.Credential {
	return access.NewContractCreds(id, ContractName, credentialAllCommand)
}

// RegisterContract registers the value contract to the given execution service.
func RegisterContract(exec *native.Service, c Contract) {
	exec.Set(ContractName, c)
}

// Contract is a smart contract that allows one to execute evoting commands
//
// - implements native.Contract
type Contract struct {

	// access is the access control service managing this smart contract
	access access.Service

	// accessKey is the access identifier allowed to use this smart contract
	accessKey []byte

	cmd commands

	pedersen dkg.DKG
}

// NewContract creates a new Value contract
func NewContract(aKey []byte, srvc access.Service, pedersen dkg.DKG) Contract {
	contract := Contract{
		// indexElection:     map[string]struct{}{},
		access:    srvc,
		accessKey: aKey,
		pedersen:  pedersen,
	}

	contract.cmd = evotingCommand{Contract: &contract, prover: proof.HashVerify}

	return contract
}

func (c Contract) Execute(snap store.Snapshot, step execution.Step) error {
	creds := NewCreds(c.accessKey)

	err := c.access.Match(snap, creds, step.Current.GetIdentity())
	if err != nil {
		return xerrors.Errorf("identity not authorized: %v (%v)",
			step.Current.GetIdentity(), err)
	}

	cmd := step.Current.GetArg(CmdArg)
	if len(cmd) == 0 {
		return xerrors.Errorf(errArgNotFound, CmdArg)
	}

	switch Command(cmd) {
	case CmdCreateElection:
		dkgActor, err := c.pedersen.GetLastActor()
		if err != nil {
			return xerrors.Errorf("failed to get dkgActor: %v", err)
		}
		err = c.cmd.createElection(snap, step, dkgActor)
		if err != nil {
			return xerrors.Errorf("failed to create election: %v", err)
		}
	case CmdCastVote:
		err := c.cmd.castVote(snap, step)
		if err != nil {
			return xerrors.Errorf("failed to cast vote: %v", err)
		}
	case CmdCloseElection:
		err := c.cmd.closeElection(snap, step)
		if err != nil {
			return xerrors.Errorf("failed to close election: %v", err)
		}
	case CmdShuffleBallots:
		err := c.cmd.shuffleBallots(snap, step)
		if err != nil {
			return xerrors.Errorf("failed to shuffle ballots: %v", err)
		}
	case CmdDecryptBallots:
		err := c.cmd.decryptBallots(snap, step)
		if err != nil {
			return xerrors.Errorf("failed to decrypt ballots: %v", err)
		}
	case CmdCancelElection:
		err := c.cmd.cancelElection(snap, step)
		if err != nil {
			return xerrors.Errorf("failed to cancel election: %v", err)
		}
	default:
		return xerrors.Errorf("unknown command: %s", cmd)
	}

	return nil
}
