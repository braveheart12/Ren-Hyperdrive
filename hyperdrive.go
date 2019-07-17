package hyperdrive

import (
	"time"

	"github.com/renproject/hyperdrive/block"
	"github.com/renproject/hyperdrive/replica"
	"github.com/renproject/hyperdrive/shard"
	"github.com/renproject/hyperdrive/sig"
	"github.com/renproject/hyperdrive/state"
	"github.com/renproject/hyperdrive/tx"
)

// NumHistoricalShards specifies the number of historical shards allowed.
const NumHistoricalShards = 3

// Hyperdrive accepts blocks and ticks and sends relevant Transitions to the respective replica.
type Hyperdrive interface {
	Sync(shardHash sig.Hash, commit block.Commit) bool

	AcceptTick(t time.Time)
	AcceptPropose(shardHash sig.Hash, proposed block.SignedPropose)
	AcceptPreVote(shardHash sig.Hash, preVote block.SignedPreVote)
	AcceptPreCommit(shardHash sig.Hash, preCommit block.SignedPreCommit)

	BeginShard(shard, previousShard shard.Shard, head *block.Commit, pool tx.Pool)
	EndShard(shardHash sig.Hash)
	DropShard(shardHash sig.Hash)
}

type hyperdrive struct {
	signer     sig.SignerVerifier
	dispatcher replica.Dispatcher

	shardReplicas map[sig.Hash]replica.Replica
}

// New returns a Hyperdrive.
func New(signer sig.SignerVerifier, dispatcher replica.Dispatcher) Hyperdrive {
	return &hyperdrive{
		signer:     signer,
		dispatcher: dispatcher,

		shardReplicas: map[sig.Hash]replica.Replica{},
	}
}

func (hyperdrive *hyperdrive) Sync(shardHash sig.Hash, commit block.Commit) bool {
	if replica, ok := hyperdrive.shardReplicas[shardHash]; ok {
		return replica.Sync(commit)
	}
	return false
}

func (hyperdrive *hyperdrive) AcceptTick(t time.Time) {
	// 1. Increment number of ticks seen by each shard
	for shardHash := range hyperdrive.shardReplicas {

		// 2. Send a Ticked transition to the shard
		if replica, ok := hyperdrive.shardReplicas[shardHash]; ok {
			replica.Transition(state.Ticked{Time: t})
		}
	}
}

func (hyperdrive *hyperdrive) AcceptPropose(shardHash sig.Hash, proposed block.SignedPropose) {
	if replica, ok := hyperdrive.shardReplicas[shardHash]; ok {
		replica.Transition(state.Proposed{SignedPropose: proposed})
	}
}

func (hyperdrive *hyperdrive) AcceptPreVote(shardHash sig.Hash, preVote block.SignedPreVote) {
	if replica, ok := hyperdrive.shardReplicas[shardHash]; ok {
		replica.Transition(state.PreVoted{SignedPreVote: preVote})
	}
}

func (hyperdrive *hyperdrive) AcceptPreCommit(shardHash sig.Hash, preCommit block.SignedPreCommit) {
	if replica, ok := hyperdrive.shardReplicas[shardHash]; ok {
		replica.Transition(state.PreCommitted{SignedPreCommit: preCommit})
	}
}

func (hyperdrive *hyperdrive) BeginShard(shard, previousShard shard.Shard, head *block.Commit, pool tx.Pool) {
	if _, ok := hyperdrive.shardReplicas[shard.Hash]; ok {
		return
	}

	r := replica.New(
		hyperdrive.dispatcher,
		hyperdrive.signer,
		state.NewMachine(state.WaitingForPropose{}, block.NewPolkaBuilder(), block.NewCommitBuilder(), hyperdrive.signer, shard, pool, head),
		state.NewTransitionBuffer(shard.Size()),
		shard,
		previousShard,
	)

	hyperdrive.shardReplicas[shard.Hash] = r

	r.Init()
}

func (hyperdrive *hyperdrive) EndShard(shardHahs sig.Hash) {
	// TODO: Stop the replica from pre-voting on blocks that contain
	// transactions. It will only pre-vote on blocks that contain the "end
	// shard" transaction.
}

func (hyperdrive *hyperdrive) DropShard(shardHash sig.Hash) {
	delete(hyperdrive.shardReplicas, shardHash)
}
