// Structured, versioned event types for efficient off-chain indexing and querying.
//
// Design principles:
// - Topics (first arg to publish) carry indexed dimensions that event consumers filter on.
//   Up to 4 topics allowed by the Soroban host.  Topic 0 is always the event-name Symbol.
// - Data payload (second arg) is a typed struct that carries every field consumers need
//   to reconstruct state without further RPC calls.
// - Every struct carries `version: u32` so consumers can detect schema changes without
//   re-reading all historical events.

use soroban_sdk::{contracttype, Address, BytesN};

/// Bumped whenever a payload struct gains or removes fields.
pub const EVENT_VERSION: u32 = 1;

// ---------------------------------------------------------------------------
// Governance events
// ---------------------------------------------------------------------------

/// Payload for "proposal_created".
/// Topics: (Symbol("proposal_created"), proposal_id, asset_id)
#[contracttype]
#[derive(Clone)]
pub struct ProposalCreatedEvent {
    pub version: u32,
    pub proposer: Address,
    pub end_time: u64,
}

/// Payload for "vote_cast".
/// Topics: (Symbol("vote_cast"), proposal_id)
#[contracttype]
#[derive(Clone)]
pub struct VoteCastEvent {
    pub version: u32,
    pub voter: Address,
    pub vote_yes: bool,
    pub weight: i128,
}

/// Payload for "proposal_executed" and "proposal_rejected".
/// Topics: (Symbol("proposal_executed"|"proposal_rejected"), proposal_id, asset_id)
#[contracttype]
#[derive(Clone)]
pub struct ProposalFinalizedEvent {
    pub version: u32,
    pub votes_for: i128,
    pub votes_against: i128,
}

// ---------------------------------------------------------------------------
// Delegation events
// ---------------------------------------------------------------------------

/// Payload for "delegate_set" and "delegation_revoked".
/// Topics: (Symbol("delegate_set"|"delegation_revoked"), delegator)
#[contracttype]
#[derive(Clone)]
pub struct DelegationEvent {
    pub version: u32,
    pub delegatee: Address,
    pub timestamp: u64,
}

// ---------------------------------------------------------------------------
// Auction events
// ---------------------------------------------------------------------------

/// Payload for "auction_created".
/// Topics: (Symbol("auction_created"), auction_id, asset_id)
#[contracttype]
#[derive(Clone)]
pub struct AuctionCreatedEvent {
    pub version: u32,
    /// 0 = English, 1 = Dutch, 2 = SealedBid
    pub auction_type: u32,
    pub seller: Address,
    pub end_time: u64,
}

/// Payload for "bid_placed".
/// Topics: (Symbol("bid_placed"), auction_id)
#[contracttype]
#[derive(Clone)]
pub struct BidPlacedEvent {
    pub version: u32,
    pub bidder: Address,
    pub amount: i128,
    pub timestamp: u64,
}

/// Payload for "auction_settled".
/// Topics: (Symbol("auction_settled"), auction_id, asset_id)
#[contracttype]
#[derive(Clone)]
pub struct AuctionSettledEvent {
    pub version: u32,
    pub winner: Option<Address>,
    pub winning_bid: i128,
    pub timestamp: u64,
}

/// Payload for "auction_cancelled".
/// Topics: (Symbol("auction_cancelled"), auction_id)
#[contracttype]
#[derive(Clone)]
pub struct AuctionCancelledEvent {
    pub version: u32,
    pub cancelled_by: Address,
    pub timestamp: u64,
}

/// Payload for "bid_refunded".
/// Topics: (Symbol("bid_refunded"), auction_id)
#[contracttype]
#[derive(Clone)]
pub struct BidRefundedEvent {
    pub version: u32,
    pub bidder: Address,
    pub amount: i128,
}

// ---------------------------------------------------------------------------
// Upgradability events
// ---------------------------------------------------------------------------

/// Payload for "upgrade_proposed".
/// Topics: (Symbol("upgrade_proposed"), proposer)
#[contracttype]
#[derive(Clone)]
pub struct UpgradeProposedEvent {
    pub version: u32,
    pub new_wasm_hash: BytesN<32>,
    pub execute_after: u64,
}

/// Payload for "upgrade_executed".
/// Topics: (Symbol("upgrade_executed"), executor)
#[contracttype]
#[derive(Clone)]
pub struct UpgradeExecutedEvent {
    pub version: u32,
    pub new_version: u32,
    pub timestamp: u64,
}

/// Payload for "upgrade_cancelled".
/// Topics: (Symbol("upgrade_cancelled"), admin)
#[contracttype]
#[derive(Clone)]
pub struct UpgradeCancelledEvent {
    pub version: u32,
    pub timestamp: u64,
}
