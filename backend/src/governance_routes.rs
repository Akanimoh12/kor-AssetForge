// governance_routes.rs — Express-style backend stubs for governance API
// These handlers bridge the frontend to the Soroban Governance contract.
//
// POST /api/governance/proposals       → contract: create_proposal
// GET  /api/governance/proposals       → contract: get_proposal (paginated)
// POST /api/governance/proposals/:id/vote → contract: vote
// POST /api/governance/delegate        → contract: delegate
// POST /api/governance/delegate/revoke → contract: revoke_delegation
// GET  /api/governance/delegate/:addr  → contract: get_delegate + get_delegated_power

// NOTE: Replace CONTRACT_ID and SOROBAN_RPC_URL with environment variables
// before deploying. All state-mutating calls must be signed by the caller
// via Freighter on the frontend and submitted as signed XDR to the backend.

use soroban_sdk::{Address, Env, String as SorobanString};
use crate::governance::{Governance, GovernanceClient};

/// Helper: invoke create_proposal on the Governance contract.
/// In production, the frontend signs the transaction with Freighter.
pub fn api_create_proposal(
    env: &Env,
    gov: &GovernanceClient,
    proposer: Address,
    asset_id: u64,
    description: SorobanString,
    duration_seconds: u64,
) -> u64 {
    gov.create_proposal(&proposer, &asset_id, &description, &duration_seconds)
}

/// Helper: invoke vote on the Governance contract.
pub fn api_vote(env: &Env, gov: &GovernanceClient, voter: Address, proposal_id: u64, support: bool) {
    gov.vote(&voter, &proposal_id, &support);
}

/// Helper: delegate voting power.
pub fn api_delegate(env: &Env, gov: &GovernanceClient, delegator: Address, delegatee: Address) {
    gov.delegate(&delegator, &delegatee);
}

/// Helper: revoke delegation.
pub fn api_revoke_delegation(env: &Env, gov: &GovernanceClient, delegator: Address) {
    gov.revoke_delegation(&delegator);
}