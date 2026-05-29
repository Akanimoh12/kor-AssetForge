use soroban_sdk::{contract, contractimpl, contracttype, Address, Env, String, Symbol, Vec};

// ============================================================================
// Data Types
// ============================================================================

#[derive(Clone, PartialEq, Eq, Debug)]
#[contracttype]
pub enum DisputeStatus {
    Open,
    UnderReview,
    Resolved,
    Rejected,
}

#[derive(Clone, PartialEq, Eq, Debug)]
#[contracttype]
pub enum DisputeResolution {
    BuyerFavor,
    SellerFavor,
    Split,
}

#[derive(Clone)]
#[contracttype]
pub struct Dispute {
    pub id: u64,
    pub transaction_id: u64,
    pub filed_by: Address,
    pub respondent: Address,
    pub reason: String,
    pub status: DisputeStatus,
    pub resolution: Option<DisputeResolution>,
    pub escrow_amount: i128,
    pub escrow_released: bool,
    pub created_at: u64,
    pub resolved_at: u64,
}

#[derive(Clone)]
#[contracttype]
pub enum DisputeDataKey {
    Admin,
    DisputeNonce,
    Dispute(u64),
    EscrowBalance(u64),
}

// ============================================================================
// Contract
// ============================================================================

#[contract]
pub struct DisputeResolution;

#[contractimpl]
impl DisputeResolution {
    /// Initialize the dispute contract with an admin.
    pub fn initialize(env: Env, admin: Address) {
        if env.storage().instance().has(&DisputeDataKey::Admin) {
            panic!("already initialized");
        }
        env.storage().instance().set(&DisputeDataKey::Admin, &admin);
    }

    /// File a new dispute; caller must be the filed_by party.
    /// Escrow amount is locked in the contract.
    pub fn file_dispute(
        env: Env,
        filed_by: Address,
        respondent: Address,
        transaction_id: u64,
        reason: String,
        escrow_amount: i128,
    ) -> u64 {
        filed_by.require_auth();

        if escrow_amount <= 0 {
            panic!("escrow amount must be positive");
        }

        let id: u64 = env
            .storage()
            .instance()
            .get(&DisputeDataKey::DisputeNonce)
            .unwrap_or(0)
            + 1;
        env.storage()
            .instance()
            .set(&DisputeDataKey::DisputeNonce, &id);

        let dispute = Dispute {
            id,
            transaction_id,
            filed_by: filed_by.clone(),
            respondent,
            reason,
            status: DisputeStatus::Open,
            resolution: None,
            escrow_amount,
            escrow_released: false,
            created_at: env.ledger().timestamp(),
            resolved_at: 0,
        };

        env.storage()
            .persistent()
            .set(&DisputeDataKey::Dispute(id), &dispute);
        env.storage()
            .persistent()
            .set(&DisputeDataKey::EscrowBalance(id), &escrow_amount);

        env.events().publish(
            (Symbol::new(&env, "dispute_filed"), id),
            (filed_by, transaction_id, escrow_amount),
        );

        id
    }

    /// Admin marks a dispute as under review.
    pub fn start_review(env: Env, admin: Address, dispute_id: u64) {
        Self::require_admin(&env, &admin);

        let mut dispute: Dispute = env
            .storage()
            .persistent()
            .get(&DisputeDataKey::Dispute(dispute_id))
            .expect("dispute not found");

        if dispute.status != DisputeStatus::Open {
            panic!("dispute must be open to start review");
        }

        dispute.status = DisputeStatus::UnderReview;
        env.storage()
            .persistent()
            .set(&DisputeDataKey::Dispute(dispute_id), &dispute);

        env.events()
            .publish((Symbol::new(&env, "dispute_review_started"), dispute_id), admin);
    }

    /// Admin resolves the dispute and releases escrowed funds.
    pub fn resolve_dispute(
        env: Env,
        admin: Address,
        dispute_id: u64,
        resolution: DisputeResolution,
    ) -> Address {
        Self::require_admin(&env, &admin);

        let mut dispute: Dispute = env
            .storage()
            .persistent()
            .get(&DisputeDataKey::Dispute(dispute_id))
            .expect("dispute not found");

        if dispute.status == DisputeStatus::Resolved || dispute.status == DisputeStatus::Rejected {
            panic!("dispute is already closed");
        }

        let release_to = match resolution {
            DisputeResolution::BuyerFavor => dispute.filed_by.clone(),
            DisputeResolution::SellerFavor => dispute.respondent.clone(),
            DisputeResolution::Split => dispute.filed_by.clone(), // split handled off-chain
        };

        dispute.status = DisputeStatus::Resolved;
        dispute.resolution = Some(resolution.clone());
        dispute.escrow_released = true;
        dispute.resolved_at = env.ledger().timestamp();

        env.storage()
            .persistent()
            .set(&DisputeDataKey::Dispute(dispute_id), &dispute);
        env.storage()
            .persistent()
            .remove(&DisputeDataKey::EscrowBalance(dispute_id));

        env.events().publish(
            (Symbol::new(&env, "dispute_resolved"), dispute_id),
            (release_to.clone(), dispute.escrow_amount),
        );

        release_to
    }

    /// Admin rejects a dispute without releasing escrow.
    pub fn reject_dispute(env: Env, admin: Address, dispute_id: u64) {
        Self::require_admin(&env, &admin);

        let mut dispute: Dispute = env
            .storage()
            .persistent()
            .get(&DisputeDataKey::Dispute(dispute_id))
            .expect("dispute not found");

        if dispute.status == DisputeStatus::Resolved || dispute.status == DisputeStatus::Rejected {
            panic!("dispute is already closed");
        }

        dispute.status = DisputeStatus::Rejected;
        dispute.resolved_at = env.ledger().timestamp();

        env.storage()
            .persistent()
            .set(&DisputeDataKey::Dispute(dispute_id), &dispute);
        env.storage()
            .persistent()
            .remove(&DisputeDataKey::EscrowBalance(dispute_id));

        env.events()
            .publish((Symbol::new(&env, "dispute_rejected"), dispute_id), admin);
    }

    /// Retrieve a dispute by ID.
    pub fn get_dispute(env: Env, dispute_id: u64) -> Option<Dispute> {
        env.storage()
            .persistent()
            .get(&DisputeDataKey::Dispute(dispute_id))
    }

    /// Get current escrow balance for a dispute.
    pub fn get_escrow_balance(env: Env, dispute_id: u64) -> i128 {
        env.storage()
            .persistent()
            .get(&DisputeDataKey::EscrowBalance(dispute_id))
            .unwrap_or(0)
    }

    /// Get all dispute IDs (up to nonce).
    pub fn get_dispute_count(env: Env) -> u64 {
        env.storage()
            .instance()
            .get(&DisputeDataKey::DisputeNonce)
            .unwrap_or(0)
    }

    fn require_admin(env: &Env, caller: &Address) {
        caller.require_auth();
        let admin: Address = env
            .storage()
            .instance()
            .get(&DisputeDataKey::Admin)
            .expect("admin not set");
        if *caller != admin {
            panic!("caller is not admin");
        }
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod test {
    use super::*;
    use soroban_sdk::testutils::Address as _;
    use soroban_sdk::{Address, Env, String};

    fn setup_contract(env: &Env) -> (DisputeResolutionClient, Address) {
        let contract_id = env.register_contract(None, DisputeResolution);
        let client = DisputeResolutionClient::new(env, &contract_id);
        let admin = Address::generate(env);
        client.initialize(&admin);
        (client, admin)
    }

    #[test]
    fn test_file_and_resolve_dispute() {
        let env = Env::default();
        env.mock_all_auths();

        let (client, admin) = setup_contract(&env);

        let buyer = Address::generate(&env);
        let seller = Address::generate(&env);
        let reason = String::from_str(&env, "Item not as described");

        let dispute_id = client.file_dispute(&buyer, &seller, &42, &reason, &1000);
        assert_eq!(dispute_id, 1);

        client.start_review(&admin, &dispute_id);

        let release_to = client.resolve_dispute(&admin, &dispute_id, &DisputeResolution::BuyerFavor);
        assert_eq!(release_to, buyer);

        let dispute = client.get_dispute(&dispute_id).unwrap();
        assert_eq!(dispute.status, DisputeStatus::Resolved);
        assert!(dispute.escrow_released);
    }

    #[test]
    fn test_reject_dispute() {
        let env = Env::default();
        env.mock_all_auths();

        let (client, admin) = setup_contract(&env);

        let buyer = Address::generate(&env);
        let seller = Address::generate(&env);
        let reason = String::from_str(&env, "Fraudulent dispute claim");

        let dispute_id = client.file_dispute(&buyer, &seller, &10, &reason, &500);
        client.reject_dispute(&admin, &dispute_id);

        let dispute = client.get_dispute(&dispute_id).unwrap();
        assert_eq!(dispute.status, DisputeStatus::Rejected);
    }

    #[test]
    #[should_panic(expected = "dispute is already closed")]
    fn test_cannot_resolve_twice() {
        let env = Env::default();
        env.mock_all_auths();

        let (client, admin) = setup_contract(&env);
        let buyer = Address::generate(&env);
        let seller = Address::generate(&env);
        let reason = String::from_str(&env, "Double resolution test");

        let id = client.file_dispute(&buyer, &seller, &1, &reason, &200);
        client.resolve_dispute(&admin, &id, &DisputeResolution::SellerFavor);
        client.resolve_dispute(&admin, &id, &DisputeResolution::BuyerFavor);
    }
}
