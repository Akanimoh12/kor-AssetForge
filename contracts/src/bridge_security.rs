use soroban_sdk::{contract, contractimpl, contracttype, Address, Env, Symbol, Vec};

// ---------------------------------------------------------------------------
// Storage Keys
// ---------------------------------------------------------------------------

#[derive(Clone)]
#[contracttype]
pub enum BridgeDataKey {
    /// Administrator address
    Admin,
    /// Set of authorized relayer addresses
    Relayers,
    /// Required number of relayer signatures to approve a bridge request
    RequiredSigs,
    /// Whether the bridge is paused
    Paused,
    /// Maximum amount that can be bridged in a single transaction
    MaxBridgeAmount,
    /// Per-user rate limit window in seconds
    RateLimitWindow,
    /// Maximum bridge requests per user per window
    RateLimitMax,
    /// Cooldown period in seconds between bridge requests for the same user
    CooldownSeconds,
    /// Timestamp of last bridge request per user
    LastBridgeTime(Address),
    /// Request count for a user in the current window
    RequestCount(Address),
    /// Start of the current rate-limit window for a user
    WindowStart(Address),
    /// Pending multi-sig approval tracking: request_id → approval count
    PendingApprovals(u64),
    /// Approvals bitmap: which relayers approved request_id
    ApprovalSigners(u64),
    /// Auto-incrementing bridge request counter
    NextRequestId,
    /// Bridge request records
    Request(u64),
    /// Total volume bridged (for analytics)
    TotalVolume,
    /// Emergency withdrawal destination
    EmergencyRecipient,
}

// ---------------------------------------------------------------------------
// Data Structures
// ---------------------------------------------------------------------------

/// A bridge request submitted by a user.
#[derive(Clone)]
#[contracttype]
pub struct BridgeRequest {
    pub id: u64,
    pub sender: Address,
    pub recipient: Address,
    pub amount: i128,
    pub source_chain: Symbol,
    pub dest_chain: Symbol,
    pub timestamp: u64,
    pub approved: bool,
    pub executed: bool,
}

/// Configuration snapshot returned for monitoring.
#[derive(Clone)]
#[contracttype]
pub struct BridgeConfig {
    pub required_sigs: u32,
    pub max_bridge_amount: i128,
    pub rate_limit_window: u64,
    pub rate_limit_max: u32,
    pub cooldown_seconds: u64,
    pub paused: bool,
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn get_admin(env: &Env) -> Address {
    env.storage()
        .instance()
        .get::<BridgeDataKey, Address>(&BridgeDataKey::Admin)
        .expect("Bridge: admin not set")
}

fn require_admin(env: &Env, caller: &Address) {
    caller.require_auth();
    if *caller != get_admin(env) {
        panic!("Bridge: caller is not admin");
    }
}

fn get_relayers(env: &Env) -> Vec<Address> {
    env.storage()
        .instance()
        .get::<BridgeDataKey, Vec<Address>>(&BridgeDataKey::Relayers)
        .unwrap_or_else(|| Vec::new(env))
}

fn is_relayer(env: &Env, addr: &Address) -> bool {
    let relayers = get_relayers(env);
    for i in 0..relayers.len() {
        if relayers.get(i).unwrap() == *addr {
            return true;
        }
    }
    false
}

fn require_not_paused(env: &Env) {
    let paused: bool = env
        .storage()
        .instance()
        .get::<BridgeDataKey, bool>(&BridgeDataKey::Paused)
        .unwrap_or(false);
    if paused {
        panic!("Bridge: contract is paused");
    }
}

fn next_request_id(env: &Env) -> u64 {
    let id: u64 = env
        .storage()
        .instance()
        .get::<BridgeDataKey, u64>(&BridgeDataKey::NextRequestId)
        .unwrap_or(0);
    env.storage()
        .instance()
        .set(&BridgeDataKey::NextRequestId, &(id + 1));
    id
}

// ---------------------------------------------------------------------------
// Contract
// ---------------------------------------------------------------------------

#[contract]
pub struct BridgeSecurity;

#[contractimpl]
impl BridgeSecurity {
    /// Initialize the bridge with security parameters.
    pub fn initialize(
        env: Env,
        admin: Address,
        required_sigs: u32,
        max_bridge_amount: i128,
        rate_limit_window: u64,
        rate_limit_max: u32,
        cooldown_seconds: u64,
    ) {
        admin.require_auth();
        env.storage()
            .instance()
            .set(&BridgeDataKey::Admin, &admin);
        env.storage()
            .instance()
            .set(&BridgeDataKey::Relayers, &Vec::<Address>::new(&env));
        env.storage()
            .instance()
            .set(&BridgeDataKey::RequiredSigs, &required_sigs);
        env.storage()
            .instance()
            .set(&BridgeDataKey::MaxBridgeAmount, &max_bridge_amount);
        env.storage()
            .instance()
            .set(&BridgeDataKey::RateLimitWindow, &rate_limit_window);
        env.storage()
            .instance()
            .set(&BridgeDataKey::RateLimitMax, &rate_limit_max);
        env.storage()
            .instance()
            .set(&BridgeDataKey::CooldownSeconds, &cooldown_seconds);
        env.storage()
            .instance()
            .set(&BridgeDataKey::Paused, &false);
        env.storage()
            .instance()
            .set(&BridgeDataKey::TotalVolume, &0_i128);
    }

    // --- Relayer Management ------------------------------------------------

    /// Add an authorized relayer.
    pub fn add_relayer(env: Env, caller: Address, relayer: Address) {
        require_admin(&env, &caller);
        let mut relayers = get_relayers(&env);
        if !is_relayer(&env, &relayer) {
            relayers.push_back(relayer);
            env.storage()
                .instance()
                .set(&BridgeDataKey::Relayers, &relayers);
        }
    }

    /// Remove a relayer (e.g., after fraud detection).
    pub fn remove_relayer(env: Env, caller: Address, relayer: Address) {
        require_admin(&env, &caller);
        let relayers = get_relayers(&env);
        let mut updated: Vec<Address> = Vec::new(&env);
        for i in 0..relayers.len() {
            let r = relayers.get(i).unwrap();
            if r != relayer {
                updated.push_back(r);
            }
        }
        env.storage()
            .instance()
            .set(&BridgeDataKey::Relayers, &updated);
    }

    // --- Bridge Requests ---------------------------------------------------

    /// Submit a new bridge request. Enforces:
    ///   - Bridge not paused
    ///   - Amount ≤ max_bridge_amount
    ///   - Per-user cooldown
    ///   - Per-user rate limit within the current window
    pub fn request_bridge(
        env: Env,
        sender: Address,
        recipient: Address,
        amount: i128,
        source_chain: Symbol,
        dest_chain: Symbol,
    ) -> u64 {
        sender.require_auth();
        require_not_paused(&env);

        // Maximum amount check.
        let max: i128 = env
            .storage()
            .instance()
            .get::<BridgeDataKey, i128>(&BridgeDataKey::MaxBridgeAmount)
            .unwrap_or(i128::MAX);
        if amount > max {
            panic!("Bridge: amount exceeds maximum bridge limit");
        }

        let now = env.ledger().timestamp();

        // Cooldown check.
        let cooldown: u64 = env
            .storage()
            .instance()
            .get::<BridgeDataKey, u64>(&BridgeDataKey::CooldownSeconds)
            .unwrap_or(0);
        let last_time: u64 = env
            .storage()
            .persistent()
            .get::<BridgeDataKey, u64>(&BridgeDataKey::LastBridgeTime(sender.clone()))
            .unwrap_or(0);
        if now < last_time + cooldown {
            panic!("Bridge: cooldown period has not elapsed");
        }

        // Rate limit check (sliding window).
        let window: u64 = env
            .storage()
            .instance()
            .get::<BridgeDataKey, u64>(&BridgeDataKey::RateLimitWindow)
            .unwrap_or(3600);
        let limit: u32 = env
            .storage()
            .instance()
            .get::<BridgeDataKey, u32>(&BridgeDataKey::RateLimitMax)
            .unwrap_or(10);

        let win_start: u64 = env
            .storage()
            .persistent()
            .get::<BridgeDataKey, u64>(&BridgeDataKey::WindowStart(sender.clone()))
            .unwrap_or(0);
        let mut count: u32 = env
            .storage()
            .persistent()
            .get::<BridgeDataKey, u32>(&BridgeDataKey::RequestCount(sender.clone()))
            .unwrap_or(0);

        if now >= win_start + window {
            // New window — reset counter.
            count = 0;
            env.storage()
                .persistent()
                .set(&BridgeDataKey::WindowStart(sender.clone()), &now);
        }

        if count >= limit {
            panic!("Bridge: rate limit exceeded for this user");
        }

        count += 1;
        env.storage()
            .persistent()
            .set(&BridgeDataKey::RequestCount(sender.clone()), &count);
        env.storage()
            .persistent()
            .set(&BridgeDataKey::LastBridgeTime(sender.clone()), &now);

        let request_id = next_request_id(&env);
        let request = BridgeRequest {
            id: request_id,
            sender: sender.clone(),
            recipient,
            amount,
            source_chain,
            dest_chain,
            timestamp: now,
            approved: false,
            executed: false,
        };

        env.storage()
            .persistent()
            .set(&BridgeDataKey::Request(request_id), &request);
        env.storage()
            .instance()
            .set(&BridgeDataKey::PendingApprovals(request_id), &0_u32);
        env.storage()
            .instance()
            .set(&BridgeDataKey::ApprovalSigners(request_id), &Vec::<Address>::new(&env));

        env.events().publish(
            (Symbol::new(&env, "bridge_requested"),),
            (request_id, sender, amount),
        );

        request_id
    }

    /// A relayer approves a bridge request. Once `required_sigs` approvals
    /// accumulate the request is marked approved.
    pub fn approve_request(env: Env, relayer: Address, request_id: u64) {
        relayer.require_auth();
        require_not_paused(&env);

        if !is_relayer(&env, &relayer) {
            panic!("Bridge: caller is not an authorized relayer");
        }

        // Duplicate signer check.
        let mut signers: Vec<Address> = env
            .storage()
            .instance()
            .get::<BridgeDataKey, Vec<Address>>(&BridgeDataKey::ApprovalSigners(request_id))
            .unwrap_or_else(|| Vec::new(&env));
        for i in 0..signers.len() {
            if signers.get(i).unwrap() == relayer {
                panic!("Bridge: relayer already approved this request");
            }
        }
        signers.push_back(relayer.clone());
        env.storage()
            .instance()
            .set(&BridgeDataKey::ApprovalSigners(request_id), &signers);

        let approvals: u32 = env
            .storage()
            .instance()
            .get::<BridgeDataKey, u32>(&BridgeDataKey::PendingApprovals(request_id))
            .unwrap_or(0)
            + 1;
        env.storage()
            .instance()
            .set(&BridgeDataKey::PendingApprovals(request_id), &approvals);

        let required: u32 = env
            .storage()
            .instance()
            .get::<BridgeDataKey, u32>(&BridgeDataKey::RequiredSigs)
            .unwrap_or(1);

        if approvals >= required {
            let mut req: BridgeRequest = env
                .storage()
                .persistent()
                .get::<BridgeDataKey, BridgeRequest>(&BridgeDataKey::Request(request_id))
                .expect("Bridge: request not found");
            req.approved = true;
            env.storage()
                .persistent()
                .set(&BridgeDataKey::Request(request_id), &req);

            // Update analytics.
            let vol: i128 = env
                .storage()
                .instance()
                .get::<BridgeDataKey, i128>(&BridgeDataKey::TotalVolume)
                .unwrap_or(0);
            env.storage()
                .instance()
                .set(&BridgeDataKey::TotalVolume, &(vol + req.amount));

            env.events().publish(
                (Symbol::new(&env, "bridge_approved"),),
                (request_id, approvals),
            );
        }
    }

    // --- Pause / Emergency -------------------------------------------------

    /// Pause or unpause the bridge. Only admin may call this.
    pub fn set_paused(env: Env, caller: Address, paused: bool) {
        require_admin(&env, &caller);
        env.storage()
            .instance()
            .set(&BridgeDataKey::Paused, &paused);
        env.events().publish(
            (Symbol::new(&env, "bridge_pause_changed"),),
            (paused, caller),
        );
    }

    /// Emergency withdrawal: send accumulated funds to a pre-configured
    /// recipient. Only callable by admin while bridge is paused.
    pub fn emergency_withdraw(env: Env, caller: Address, amount: i128) {
        require_admin(&env, &caller);
        let paused: bool = env
            .storage()
            .instance()
            .get::<BridgeDataKey, bool>(&BridgeDataKey::Paused)
            .unwrap_or(false);
        if !paused {
            panic!("Bridge: must be paused for emergency withdrawal");
        }
        let recipient: Address = env
            .storage()
            .instance()
            .get::<BridgeDataKey, Address>(&BridgeDataKey::EmergencyRecipient)
            .expect("Bridge: emergency recipient not set");
        env.events().publish(
            (Symbol::new(&env, "emergency_withdrawal"),),
            (amount, recipient, caller),
        );
    }

    /// Set the emergency withdrawal destination address.
    pub fn set_emergency_recipient(env: Env, caller: Address, recipient: Address) {
        require_admin(&env, &caller);
        env.storage()
            .instance()
            .set(&BridgeDataKey::EmergencyRecipient, &recipient);
    }

    // --- Config Getters ----------------------------------------------------

    /// Return a snapshot of the current bridge configuration.
    pub fn get_config(env: Env) -> BridgeConfig {
        BridgeConfig {
            required_sigs: env
                .storage()
                .instance()
                .get::<BridgeDataKey, u32>(&BridgeDataKey::RequiredSigs)
                .unwrap_or(1),
            max_bridge_amount: env
                .storage()
                .instance()
                .get::<BridgeDataKey, i128>(&BridgeDataKey::MaxBridgeAmount)
                .unwrap_or(0),
            rate_limit_window: env
                .storage()
                .instance()
                .get::<BridgeDataKey, u64>(&BridgeDataKey::RateLimitWindow)
                .unwrap_or(3600),
            rate_limit_max: env
                .storage()
                .instance()
                .get::<BridgeDataKey, u32>(&BridgeDataKey::RateLimitMax)
                .unwrap_or(10),
            cooldown_seconds: env
                .storage()
                .instance()
                .get::<BridgeDataKey, u64>(&BridgeDataKey::CooldownSeconds)
                .unwrap_or(0),
            paused: env
                .storage()
                .instance()
                .get::<BridgeDataKey, bool>(&BridgeDataKey::Paused)
                .unwrap_or(false),
        }
    }

    /// Return total bridged volume for analytics.
    pub fn get_total_volume(env: Env) -> i128 {
        env.storage()
            .instance()
            .get::<BridgeDataKey, i128>(&BridgeDataKey::TotalVolume)
            .unwrap_or(0)
    }

    /// Fetch a specific bridge request by ID.
    pub fn get_request(env: Env, request_id: u64) -> BridgeRequest {
        env.storage()
            .persistent()
            .get::<BridgeDataKey, BridgeRequest>(&BridgeDataKey::Request(request_id))
            .expect("Bridge: request not found")
    }
}
