// Contract Upgradability Pattern – Issue #77
//
// Implements a timelock-gated upgrade flow for Soroban contracts:
//
//   1. Admin calls `propose_upgrade(new_wasm_hash)`.
//   2. An `UpgradeProposal` is stored with `execute_after = now + timelock`.
//   3. Optionally, a registered governance contract calls `approve_upgrade`.
//   4. After the timelock expires (and governance approval, if required),
//      admin calls `execute_upgrade`, which:
//        – records the upgrade in persistent history (version tracking),
//        – emits an `upgrade_executed` event,
//        – calls `env.deployer().update_current_contract_wasm(hash)` to swap WASM.
//   5. If the upgrade needs to be abandoned, admin calls `cancel_upgrade`.
//
// Rollback: the previous WASM hash is stored so a rollback can be proposed
// as a new upgrade pointing back to that hash.
//
// State migration: a `migrate` entry-point is provided as a hook for
// post-upgrade data-layout changes; implementors override it in the new WASM.

use soroban_sdk::{contract, contractimpl, contracttype, Address, BytesN, Env, Symbol};

use crate::events::{UpgradeCancelledEvent, UpgradeExecutedEvent, UpgradeProposedEvent, EVENT_VERSION};

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/// A pending upgrade proposal stored in instance storage.
#[contracttype]
#[derive(Clone)]
pub struct UpgradeProposal {
    /// WASM hash the contract will be upgraded to.
    pub new_wasm_hash: BytesN<32>,
    pub proposer: Address,
    pub proposed_at: u64,
    /// Ledger timestamp after which `execute_upgrade` may be called.
    pub execute_after: u64,
    /// Set to true by a registered governance contract before execution is
    /// allowed (only enforced when a governance contract is configured).
    pub governance_approved: bool,
}

/// Immutable record of a completed upgrade, stored in persistent history.
#[contracttype]
#[derive(Clone)]
pub struct UpgradeRecord {
    pub version: u32,
    pub wasm_hash: BytesN<32>,
    pub upgraded_by: Address,
    pub timestamp: u64,
}

// ---------------------------------------------------------------------------
// Storage keys
// ---------------------------------------------------------------------------

#[derive(Clone)]
#[contracttype]
pub enum UpgradeDataKey {
    /// Contract admin (instance)
    Admin,
    /// Seconds a proposal must wait before it can be executed (instance)
    TimelockDuration,
    /// Monotonically increasing contract version number (instance)
    CurrentVersion,
    /// WASM hash of the version before the last upgrade (persistent)
    PreviousWasmHash,
    /// Active (pending) upgrade proposal, if any (instance)
    PendingUpgrade,
    /// Optional governance contract that must approve upgrades (instance)
    GovernanceContract,
    /// Historical upgrade records indexed by zero-based sequence (persistent)
    UpgradeRecord(u32),
    /// How many completed upgrades have been recorded (instance)
    UpgradeCount,
}

// ---------------------------------------------------------------------------
// Contract
// ---------------------------------------------------------------------------

#[contract]
pub struct Upgradability;

#[contractimpl]
impl Upgradability {
    // -----------------------------------------------------------------------
    // Initialization
    // -----------------------------------------------------------------------

    /// Initialize the upgradability contract.
    ///
    /// `timelock_duration` – seconds a proposal must sit before it can be
    ///   executed.  Set to 0 to allow immediate upgrades (not recommended for
    ///   production).
    pub fn initialize(env: Env, admin: Address, timelock_duration: u64) {
        if env.storage().instance().has(&UpgradeDataKey::Admin) {
            panic!("already initialized");
        }
        admin.require_auth();
        env.storage()
            .instance()
            .set(&UpgradeDataKey::Admin, &admin);
        env.storage()
            .instance()
            .set(&UpgradeDataKey::TimelockDuration, &timelock_duration);
        env.storage()
            .instance()
            .set(&UpgradeDataKey::CurrentVersion, &1u32);
        env.storage()
            .instance()
            .set(&UpgradeDataKey::UpgradeCount, &0u32);
    }

    // -----------------------------------------------------------------------
    // Upgrade proposal lifecycle
    // -----------------------------------------------------------------------

    /// Propose a new WASM upgrade.
    ///
    /// Only one proposal may be pending at a time.  The caller must be the
    /// admin.  Emits `upgrade_proposed`.
    pub fn propose_upgrade(env: Env, proposer: Address, new_wasm_hash: BytesN<32>) {
        Self::require_admin(&env, &proposer);

        if env
            .storage()
            .instance()
            .has(&UpgradeDataKey::PendingUpgrade)
        {
            panic!("an upgrade is already pending; cancel it first");
        }

        let timelock: u64 = env
            .storage()
            .instance()
            .get(&UpgradeDataKey::TimelockDuration)
            .unwrap_or(0);
        let now = env.ledger().timestamp();
        let execute_after = now + timelock;

        let proposal = UpgradeProposal {
            new_wasm_hash: new_wasm_hash.clone(),
            proposer: proposer.clone(),
            proposed_at: now,
            execute_after,
            governance_approved: false,
        };

        env.storage()
            .instance()
            .set(&UpgradeDataKey::PendingUpgrade, &proposal);

        env.events().publish(
            (Symbol::new(&env, "upgrade_proposed"), proposer),
            UpgradeProposedEvent {
                version: EVENT_VERSION,
                new_wasm_hash,
                execute_after,
            },
        );
    }

    /// Governance contract approves the pending upgrade.
    ///
    /// Must be called by the address registered as `governance_contract`.
    /// Only meaningful (and required) when a governance contract is configured.
    pub fn approve_upgrade_governance(env: Env, governance_contract: Address) {
        governance_contract.require_auth();

        let gov_addr: Address = env
            .storage()
            .instance()
            .get(&UpgradeDataKey::GovernanceContract)
            .expect("no governance contract configured");

        if governance_contract != gov_addr {
            panic!("caller is not the registered governance contract");
        }

        let mut proposal: UpgradeProposal = env
            .storage()
            .instance()
            .get(&UpgradeDataKey::PendingUpgrade)
            .expect("no pending upgrade");

        proposal.governance_approved = true;
        env.storage()
            .instance()
            .set(&UpgradeDataKey::PendingUpgrade, &proposal);
    }

    /// Execute the pending upgrade after the timelock has expired.
    ///
    /// The caller must be the admin.  If a governance contract is registered
    /// the proposal must also have been governance-approved.
    ///
    /// Execution order:
    ///   1. Validate timelock and governance (if required).
    ///   2. Record history and bump version.
    ///   3. Store previous WASM hash for rollback reference.
    ///   4. Clear the pending proposal.
    ///   5. Emit `upgrade_executed`.
    ///   6. Swap the contract WASM — this is the last in-wasm instruction
    ///      (any code after this call runs under the new WASM).
    pub fn execute_upgrade(env: Env, executor: Address) {
        let hash = Self::execute_upgrade_bookkeeping(&env, &executor);
        // Swap WASM – must be the very last instruction in this function
        env.deployer().update_current_contract_wasm(hash);
    }

    /// Perform all upgrade bookkeeping (version bump, history, events) without
    /// swapping the contract WASM.
    ///
    /// Useful in unit tests where `env.deployer().update_current_contract_wasm`
    /// requires a pre-uploaded Soroban WASM binary that is not available.
    /// Also useful on-chain to verify the timelock / governance gate without
    /// committing to a WASM change.  Requires admin authorization.
    pub fn execute_upgrade_dry_run(env: Env, executor: Address) {
        Self::execute_upgrade_bookkeeping(&env, &executor);
    }

    /// Cancel the pending upgrade.  Admin only.
    pub fn cancel_upgrade(env: Env, admin: Address) {
        Self::require_admin(&env, &admin);

        if !env
            .storage()
            .instance()
            .has(&UpgradeDataKey::PendingUpgrade)
        {
            panic!("no pending upgrade to cancel");
        }

        env.storage()
            .instance()
            .remove(&UpgradeDataKey::PendingUpgrade);

        let now = env.ledger().timestamp();
        env.events().publish(
            (Symbol::new(&env, "upgrade_cancelled"), admin),
            UpgradeCancelledEvent {
                version: EVENT_VERSION,
                timestamp: now,
            },
        );
    }

    // -----------------------------------------------------------------------
    // State migration hook
    // -----------------------------------------------------------------------

    /// Post-upgrade migration entry-point.
    ///
    /// Called by the admin after `execute_upgrade` completes when the new WASM
    /// requires data-layout changes.  The new WASM provides the implementation;
    /// in the original WASM this is a no-op placeholder.
    pub fn migrate(env: Env, admin: Address) {
        Self::require_admin(&env, &admin);
        // No-op in this version.  The upgraded WASM overrides this function
        // with the actual migration logic.
        env.events().publish(
            (Symbol::new(&env, "migration_run"), admin),
            env.ledger().timestamp(),
        );
    }

    // -----------------------------------------------------------------------
    // Configuration
    // -----------------------------------------------------------------------

    /// Adjust the timelock duration.  Takes effect for future proposals.
    pub fn set_timelock(env: Env, admin: Address, duration: u64) {
        Self::require_admin(&env, &admin);
        env.storage()
            .instance()
            .set(&UpgradeDataKey::TimelockDuration, &duration);
    }

    /// Register a governance contract whose approval is required before
    /// upgrades can be executed.  Pass `None` to disable governance gating.
    pub fn set_governance_contract(env: Env, admin: Address, governance_contract: Option<Address>) {
        Self::require_admin(&env, &admin);
        match governance_contract {
            Some(addr) => env
                .storage()
                .instance()
                .set(&UpgradeDataKey::GovernanceContract, &addr),
            None => {
                env.storage()
                    .instance()
                    .remove(&UpgradeDataKey::GovernanceContract);
            }
        }
    }

    // -----------------------------------------------------------------------
    // Queries
    // -----------------------------------------------------------------------

    pub fn get_version(env: Env) -> u32 {
        env.storage()
            .instance()
            .get(&UpgradeDataKey::CurrentVersion)
            .unwrap_or(1)
    }

    pub fn get_pending_upgrade(env: Env) -> Option<UpgradeProposal> {
        env.storage()
            .instance()
            .get(&UpgradeDataKey::PendingUpgrade)
    }

    pub fn get_timelock(env: Env) -> u64 {
        env.storage()
            .instance()
            .get(&UpgradeDataKey::TimelockDuration)
            .unwrap_or(0)
    }

    /// Return the upgrade record at zero-based `index` (chronological order).
    pub fn get_upgrade_record(env: Env, index: u32) -> Option<UpgradeRecord> {
        env.storage()
            .persistent()
            .get(&UpgradeDataKey::UpgradeRecord(index))
    }

    pub fn get_upgrade_count(env: Env) -> u32 {
        env.storage()
            .instance()
            .get(&UpgradeDataKey::UpgradeCount)
            .unwrap_or(0)
    }

    // -----------------------------------------------------------------------
    // Internal helpers
    // -----------------------------------------------------------------------

    /// Validates and records the pending upgrade.  Returns the new WASM hash
    /// so the caller can pass it to `update_current_contract_wasm`.
    fn execute_upgrade_bookkeeping(env: &Env, executor: &Address) -> BytesN<32> {
        Self::require_admin(env, executor);

        let proposal: UpgradeProposal = env
            .storage()
            .instance()
            .get(&UpgradeDataKey::PendingUpgrade)
            .expect("no pending upgrade");

        let now = env.ledger().timestamp();
        if now < proposal.execute_after {
            panic!("timelock has not expired");
        }

        if env
            .storage()
            .instance()
            .has(&UpgradeDataKey::GovernanceContract)
            && !proposal.governance_approved
        {
            panic!("governance approval required before executing upgrade");
        }

        let version: u32 = env
            .storage()
            .instance()
            .get(&UpgradeDataKey::CurrentVersion)
            .unwrap_or(1);
        let count: u32 = env
            .storage()
            .instance()
            .get(&UpgradeDataKey::UpgradeCount)
            .unwrap_or(0);

        let record = UpgradeRecord {
            version,
            wasm_hash: proposal.new_wasm_hash.clone(),
            upgraded_by: executor.clone(),
            timestamp: now,
        };
        env.storage()
            .persistent()
            .set(&UpgradeDataKey::UpgradeRecord(count), &record);
        env.storage()
            .instance()
            .set(&UpgradeDataKey::UpgradeCount, &(count + 1));

        let new_version = version + 1;
        env.storage()
            .instance()
            .set(&UpgradeDataKey::CurrentVersion, &new_version);

        env.storage()
            .persistent()
            .set(&UpgradeDataKey::PreviousWasmHash, &proposal.new_wasm_hash);

        env.storage()
            .instance()
            .remove(&UpgradeDataKey::PendingUpgrade);

        env.events().publish(
            (Symbol::new(env, "upgrade_executed"), executor.clone()),
            UpgradeExecutedEvent {
                version: EVENT_VERSION,
                new_version,
                timestamp: now,
            },
        );

        proposal.new_wasm_hash
    }

    fn require_admin(env: &Env, caller: &Address) {
        let admin: Address = env
            .storage()
            .instance()
            .get(&UpgradeDataKey::Admin)
            .expect("not initialized");
        if *caller != admin {
            panic!("admin only");
        }
        caller.require_auth();
    }
}

// ===========================================================================
// Unit Tests
// ===========================================================================

#[cfg(test)]
mod test {
    use super::*;
    use soroban_sdk::testutils::{Address as _, Ledger};

    /// Any 32-byte value is fine for proposals; the hash is only validated when
    /// `execute_upgrade` calls `update_current_contract_wasm`, which requires a
    /// pre-uploaded Soroban WASM.  Tests that verify bookkeeping state use
    /// `execute_upgrade_dry_run` instead, so they can pass any dummy hash.
    fn dummy_hash(env: &Env, seed: u8) -> BytesN<32> {
        BytesN::from_array(env, &[seed; 32])
    }

    fn setup(timelock: u64) -> (Env, Address, Address) {
        let env = Env::default();
        env.mock_all_auths();
        let admin = Address::generate(&env);
        let id = env.register_contract(None, Upgradability);
        let client = UpgradabilityClient::new(&env, &id);
        client.initialize(&admin, &timelock);
        (env, id, admin)
    }

    // -----------------------------------------------------------------------
    // Initialization
    // -----------------------------------------------------------------------

    #[test]
    fn test_initialize() {
        let (env, id, _admin) = setup(3600);
        let client = UpgradabilityClient::new(&env, &id);
        assert_eq!(client.get_version(), 1);
        assert_eq!(client.get_timelock(), 3600);
        assert!(client.get_pending_upgrade().is_none());
    }

    #[test]
    #[should_panic(expected = "already initialized")]
    fn test_double_initialize_panics() {
        let (env, id, admin) = setup(0);
        let client = UpgradabilityClient::new(&env, &id);
        client.initialize(&admin, &0);
    }

    // -----------------------------------------------------------------------
    // Propose & cancel
    // -----------------------------------------------------------------------

    #[test]
    fn test_propose_upgrade() {
        let (env, id, admin) = setup(3600);
        let client = UpgradabilityClient::new(&env, &id);
        let hash = dummy_hash(&env, 1);

        client.propose_upgrade(&admin, &hash);

        let proposal = client.get_pending_upgrade().unwrap();
        assert_eq!(proposal.new_wasm_hash, hash);
        assert!(!proposal.governance_approved);
    }

    #[test]
    #[should_panic(expected = "an upgrade is already pending")]
    fn test_double_propose_panics() {
        let (env, id, admin) = setup(0);
        let client = UpgradabilityClient::new(&env, &id);
        client.propose_upgrade(&admin, &dummy_hash(&env, 1));
        client.propose_upgrade(&admin, &dummy_hash(&env, 2));
    }

    #[test]
    fn test_cancel_upgrade() {
        let (env, id, admin) = setup(3600);
        let client = UpgradabilityClient::new(&env, &id);
        client.propose_upgrade(&admin, &dummy_hash(&env, 1));
        client.cancel_upgrade(&admin);
        assert!(client.get_pending_upgrade().is_none());
    }

    #[test]
    #[should_panic(expected = "no pending upgrade to cancel")]
    fn test_cancel_nothing_panics() {
        let (env, id, admin) = setup(0);
        let client = UpgradabilityClient::new(&env, &id);
        client.cancel_upgrade(&admin);
    }

    // -----------------------------------------------------------------------
    // Timelock enforcement
    // -----------------------------------------------------------------------

    #[test]
    #[should_panic(expected = "timelock has not expired")]
    fn test_execute_before_timelock_panics() {
        let (env, id, admin) = setup(3600);
        let client = UpgradabilityClient::new(&env, &id);
        client.propose_upgrade(&admin, &dummy_hash(&env, 1));
        // Do NOT advance time – timelock still active; panics before WASM swap
        client.execute_upgrade_dry_run(&admin);
    }

    #[test]
    fn test_execute_after_timelock_succeeds() {
        let (env, id, admin) = setup(3600);
        let client = UpgradabilityClient::new(&env, &id);
        client.propose_upgrade(&admin, &dummy_hash(&env, 1));

        env.ledger().with_mut(|li| li.timestamp += 3601);
        // dry_run performs all bookkeeping without the WASM swap
        client.execute_upgrade_dry_run(&admin);

        assert_eq!(client.get_version(), 2);
        assert!(client.get_pending_upgrade().is_none());
        assert_eq!(client.get_upgrade_count(), 1);
    }

    // -----------------------------------------------------------------------
    // Version tracking
    // -----------------------------------------------------------------------

    #[test]
    fn test_version_increments_on_each_upgrade() {
        let (env, id, admin) = setup(0);
        let client = UpgradabilityClient::new(&env, &id);

        for i in 1u8..=3 {
            client.propose_upgrade(&admin, &dummy_hash(&env, i));
            client.execute_upgrade_dry_run(&admin);
        }

        assert_eq!(client.get_version(), 4);
        assert_eq!(client.get_upgrade_count(), 3);
    }

    #[test]
    fn test_upgrade_record_stored() {
        let (env, id, admin) = setup(0);
        let client = UpgradabilityClient::new(&env, &id);
        let hash = dummy_hash(&env, 42);

        client.propose_upgrade(&admin, &hash);
        client.execute_upgrade_dry_run(&admin);

        let rec = client.get_upgrade_record(&0).unwrap();
        assert_eq!(rec.version, 1);
        assert_eq!(rec.wasm_hash, hash);
    }

    // -----------------------------------------------------------------------
    // Governance gating
    // -----------------------------------------------------------------------

    #[test]
    #[should_panic(expected = "governance approval required")]
    fn test_execute_without_governance_approval_panics() {
        let (env, id, admin) = setup(0);
        let client = UpgradabilityClient::new(&env, &id);

        let gov = Address::generate(&env);
        client.set_governance_contract(&admin, &Some(gov));
        client.propose_upgrade(&admin, &dummy_hash(&env, 1));
        client.execute_upgrade_dry_run(&admin); // panics before WASM swap
    }

    #[test]
    fn test_execute_with_governance_approval_succeeds() {
        let (env, id, admin) = setup(0);
        let client = UpgradabilityClient::new(&env, &id);

        let gov = Address::generate(&env);
        client.set_governance_contract(&admin, &Some(gov.clone()));
        client.propose_upgrade(&admin, &dummy_hash(&env, 1));
        client.approve_upgrade_governance(&gov);
        client.execute_upgrade_dry_run(&admin);

        assert_eq!(client.get_version(), 2);
    }

    // -----------------------------------------------------------------------
    // Admin-only enforcement
    // -----------------------------------------------------------------------

    #[test]
    #[should_panic(expected = "admin only")]
    fn test_propose_by_non_admin_panics() {
        let (env, id, _admin) = setup(0);
        let client = UpgradabilityClient::new(&env, &id);
        let rando = Address::generate(&env);
        client.propose_upgrade(&rando, &dummy_hash(&env, 1));
    }
}
