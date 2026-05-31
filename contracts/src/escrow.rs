use soroban_sdk::{
    contract, contractimpl, contracttype, token, Address, Env, String,
};

// ---------------------------------------------------------------------------
// Storage Keys
// ---------------------------------------------------------------------------

#[derive(Clone)]
#[contracttype]
pub enum EscrowDataKey {
    Admin,
    NextEscrowId,
    Escrow(u64),
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

#[derive(Clone, PartialEq, Debug)]
#[contracttype]
pub enum EscrowStatus {
    Active,
    Released,
    Refunded,
    Disputed,
    Resolved,
}

#[derive(Clone)]
#[contracttype]
pub struct EscrowRecord {
    pub escrow_id: u64,
    pub buyer: Address,
    pub seller: Address,
    pub token: Address,
    pub amount: i128,
    pub asset_id: u64,
    /// Unix timestamp after which buyer can raise a dispute
    pub release_deadline: u64,
    pub status: EscrowStatus,
    pub dispute_notes: String,
    pub created_at: u64,
}

// ---------------------------------------------------------------------------
// Contract
// ---------------------------------------------------------------------------

#[contract]
pub struct Escrow;

#[contractimpl]
impl Escrow {
    /// Initialize with an admin address (used as arbiter for disputes).
    pub fn initialize(env: Env, admin: Address) {
        admin.require_auth();
        env.storage().instance().set(&EscrowDataKey::Admin, &admin);
        env.storage().instance().set(&EscrowDataKey::NextEscrowId, &1u64);
    }

    /// Buyer creates an escrow, locking funds in the contract until release conditions are met.
    pub fn create_escrow(
        env: Env,
        buyer: Address,
        seller: Address,
        token: Address,
        amount: i128,
        asset_id: u64,
        release_deadline: u64,
    ) -> u64 {
        buyer.require_auth();
        let id: u64 = env
            .storage()
            .instance()
            .get(&EscrowDataKey::NextEscrowId)
            .unwrap_or(1);

        // Transfer funds from buyer to this contract
        let token_client = token::Client::new(&env, &token);
        token_client.transfer(&buyer, &env.current_contract_address(), &amount);

        let record = EscrowRecord {
            escrow_id: id,
            buyer: buyer.clone(),
            seller: seller.clone(),
            token: token.clone(),
            amount,
            asset_id,
            release_deadline,
            status: EscrowStatus::Active,
            dispute_notes: String::from_str(&env, ""),
            created_at: env.ledger().timestamp(),
        };
        env.storage().instance().set(&EscrowDataKey::Escrow(id), &record);
        env.storage()
            .instance()
            .set(&EscrowDataKey::NextEscrowId, &(id + 1));
        id
    }

    /// Buyer confirms receipt of asset — releases funds to seller.
    pub fn release(env: Env, buyer: Address, escrow_id: u64) {
        buyer.require_auth();
        let mut rec: EscrowRecord = env
            .storage()
            .instance()
            .get(&EscrowDataKey::Escrow(escrow_id))
            .expect("escrow not found");
        assert_eq!(rec.buyer, buyer, "only buyer can release");
        assert_eq!(rec.status, EscrowStatus::Active, "escrow not active");

        let token_client = token::Client::new(&env, &rec.token);
        token_client.transfer(&env.current_contract_address(), &rec.seller, &rec.amount);
        rec.status = EscrowStatus::Released;
        env.storage()
            .instance()
            .set(&EscrowDataKey::Escrow(escrow_id), &rec);
    }

    /// Buyer raises a dispute before the release deadline.
    pub fn raise_dispute(env: Env, buyer: Address, escrow_id: u64, notes: String) {
        buyer.require_auth();
        let mut rec: EscrowRecord = env
            .storage()
            .instance()
            .get(&EscrowDataKey::Escrow(escrow_id))
            .expect("escrow not found");
        assert_eq!(rec.buyer, buyer, "only buyer can dispute");
        assert_eq!(rec.status, EscrowStatus::Active, "escrow not active");
        assert!(
            env.ledger().timestamp() <= rec.release_deadline,
            "dispute window expired"
        );
        rec.status = EscrowStatus::Disputed;
        rec.dispute_notes = notes;
        env.storage()
            .instance()
            .set(&EscrowDataKey::Escrow(escrow_id), &rec);
    }

    /// Admin resolves a disputed escrow: true = release to seller, false = refund buyer.
    pub fn resolve_dispute(env: Env, admin: Address, escrow_id: u64, release_to_seller: bool) {
        admin.require_auth();
        let stored_admin: Address = env
            .storage()
            .instance()
            .get(&EscrowDataKey::Admin)
            .unwrap();
        assert_eq!(admin, stored_admin, "unauthorized");
        let mut rec: EscrowRecord = env
            .storage()
            .instance()
            .get(&EscrowDataKey::Escrow(escrow_id))
            .expect("escrow not found");
        assert_eq!(rec.status, EscrowStatus::Disputed, "escrow not in dispute");

        let token_client = token::Client::new(&env, &rec.token);
        let recipient = if release_to_seller { rec.seller.clone() } else { rec.buyer.clone() };
        token_client.transfer(&env.current_contract_address(), &recipient, &rec.amount);
        rec.status = EscrowStatus::Resolved;
        env.storage()
            .instance()
            .set(&EscrowDataKey::Escrow(escrow_id), &rec);
    }

    /// After release_deadline, seller can claim funds if no dispute was raised.
    pub fn claim_expired(env: Env, seller: Address, escrow_id: u64) {
        seller.require_auth();
        let mut rec: EscrowRecord = env
            .storage()
            .instance()
            .get(&EscrowDataKey::Escrow(escrow_id))
            .expect("escrow not found");
        assert_eq!(rec.seller, seller, "only seller can claim");
        assert_eq!(rec.status, EscrowStatus::Active, "escrow not active");
        assert!(
            env.ledger().timestamp() > rec.release_deadline,
            "release deadline not reached"
        );
        let token_client = token::Client::new(&env, &rec.token);
        token_client.transfer(&env.current_contract_address(), &rec.seller, &rec.amount);
        rec.status = EscrowStatus::Released;
        env.storage()
            .instance()
            .set(&EscrowDataKey::Escrow(escrow_id), &rec);
    }

    pub fn get_escrow(env: Env, escrow_id: u64) -> Option<EscrowRecord> {
        env.storage().instance().get(&EscrowDataKey::Escrow(escrow_id))
    }
}