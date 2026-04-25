// Marketplace Auction System – Issue #71
//
// Supports three auction types:
//   English   – ascending price; highest bidder wins; bid extensions near deadline.
//   Dutch     – descending price; first buyer at or above current price wins.
//   SealedBid – all bids recorded; winner determined at settlement (highest bid
//               at or above reserve wins).  On-chain state is readable, so this
//               is "sealed" only in the social/UX sense.
//
// Bid refunds: outbid amounts are tracked in PendingRefund storage so bidders
// can claim them; a BidRefunded event is emitted for off-chain monitoring.

use soroban_sdk::{contract, contractimpl, contracttype, Address, Env, Symbol, Vec};

use crate::events::{
    AuctionCancelledEvent, AuctionCreatedEvent, AuctionSettledEvent, BidPlacedEvent,
    BidRefundedEvent, EVENT_VERSION,
};

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

#[contracttype]
#[derive(Clone, PartialEq, Debug)]
pub enum AuctionType {
    English,
    Dutch,
    SealedBid,
}

#[contracttype]
#[derive(Clone, PartialEq, Debug)]
pub enum AuctionStatus {
    Active,
    Ended,
    Settled,
    Cancelled,
}

#[contracttype]
#[derive(Clone)]
pub struct Auction {
    pub auction_id: u64,
    pub auction_type: AuctionType,
    pub seller: Address,
    pub asset_id: u64,
    /// Number of asset units being auctioned
    pub amount: i128,
    /// Minimum acceptable final price; auction settles without winner if not met
    pub reserve_price: i128,
    pub start_time: u64,
    pub end_time: u64,
    pub status: AuctionStatus,
    /// English/SealedBid: opening bid. Dutch: starting (highest) price.
    pub start_price: i128,
    /// English/SealedBid: current highest bid amount. Dutch: 0 until bought.
    pub highest_bid: i128,
    pub highest_bidder: Option<Address>,
    /// English: minimum amount a new bid must exceed the current highest bid by.
    pub min_bid_increment: i128,
    /// Dutch: price drop per `decrement_interval` seconds.
    pub price_decrement: i128,
    /// Dutch: how often (in seconds) the price drops by `price_decrement`.
    pub decrement_interval: u64,
    /// English: if a bid arrives within this many seconds of `end_time`,
    /// the auction is extended by `extension_window` seconds.
    pub extension_window: u64,
}

/// A bid record stored per (auction_id, bidder).
#[contracttype]
#[derive(Clone)]
pub struct Bid {
    pub bidder: Address,
    pub amount: i128,
    pub timestamp: u64,
}

// ---------------------------------------------------------------------------
// Storage keys
// ---------------------------------------------------------------------------

#[derive(Clone)]
#[contracttype]
pub enum AuctionDataKey {
    /// Contract admin address (instance)
    Admin,
    /// Auto-incrementing auction id counter (instance)
    AuctionNonce,
    /// Auction state by id (persistent)
    Auction(u64),
    /// Best bid per bidder per auction (persistent)
    Bid(u64, Address),
    /// Ordered list of all bidders per auction (persistent) – used for settlement/refunds
    Bidders(u64),
    /// Unclaimed refund balance per bidder per auction (persistent)
    PendingRefund(u64, Address),
}

// ---------------------------------------------------------------------------
// Contract
// ---------------------------------------------------------------------------

#[contract]
pub struct AuctionHouse;

#[contractimpl]
impl AuctionHouse {
    // -----------------------------------------------------------------------
    // Initialization
    // -----------------------------------------------------------------------

    pub fn initialize(env: Env, admin: Address) {
        if env.storage().instance().has(&AuctionDataKey::Admin) {
            panic!("already initialized");
        }
        admin.require_auth();
        env.storage()
            .instance()
            .set(&AuctionDataKey::Admin, &admin);
        env.storage()
            .instance()
            .set(&AuctionDataKey::AuctionNonce, &0u64);
    }

    // -----------------------------------------------------------------------
    // Create auctions
    // -----------------------------------------------------------------------

    /// Create an English (ascending-price) auction.
    ///
    /// Returns the new `auction_id`.
    pub fn create_english_auction(
        env: Env,
        seller: Address,
        asset_id: u64,
        amount: i128,
        start_price: i128,
        reserve_price: i128,
        min_bid_increment: i128,
        duration: u64,
        extension_window: u64,
    ) -> u64 {
        seller.require_auth();
        Self::validate_create(amount, start_price, reserve_price, duration);

        let auction_id = Self::next_id(&env);
        let now = env.ledger().timestamp();
        let auction = Auction {
            auction_id,
            auction_type: AuctionType::English,
            seller: seller.clone(),
            asset_id,
            amount,
            reserve_price,
            start_time: now,
            end_time: now + duration,
            status: AuctionStatus::Active,
            start_price,
            highest_bid: 0,
            highest_bidder: None,
            min_bid_increment,
            price_decrement: 0,
            decrement_interval: 0,
            extension_window,
        };

        env.storage()
            .persistent()
            .set(&AuctionDataKey::Auction(auction_id), &auction);

        env.events().publish(
            (
                Symbol::new(&env, "auction_created"),
                auction_id,
                asset_id,
            ),
            AuctionCreatedEvent {
                version: EVENT_VERSION,
                auction_type: 0,
                seller,
                end_time: now + duration,
            },
        );

        auction_id
    }

    /// Create a Dutch (descending-price) auction.
    ///
    /// Price starts at `start_price` and drops by `price_decrement` every
    /// `decrement_interval` seconds until it reaches `reserve_price` or a
    /// buyer calls `buy_dutch`.
    pub fn create_dutch_auction(
        env: Env,
        seller: Address,
        asset_id: u64,
        amount: i128,
        start_price: i128,
        reserve_price: i128,
        price_decrement: i128,
        decrement_interval: u64,
        duration: u64,
    ) -> u64 {
        seller.require_auth();
        Self::validate_create(amount, start_price, reserve_price, duration);
        if price_decrement <= 0 {
            panic!("price_decrement must be positive");
        }
        if decrement_interval == 0 {
            panic!("decrement_interval must be non-zero");
        }

        let auction_id = Self::next_id(&env);
        let now = env.ledger().timestamp();
        let auction = Auction {
            auction_id,
            auction_type: AuctionType::Dutch,
            seller: seller.clone(),
            asset_id,
            amount,
            reserve_price,
            start_time: now,
            end_time: now + duration,
            status: AuctionStatus::Active,
            start_price,
            highest_bid: 0,
            highest_bidder: None,
            min_bid_increment: 0,
            price_decrement,
            decrement_interval,
            extension_window: 0,
        };

        env.storage()
            .persistent()
            .set(&AuctionDataKey::Auction(auction_id), &auction);

        env.events().publish(
            (
                Symbol::new(&env, "auction_created"),
                auction_id,
                asset_id,
            ),
            AuctionCreatedEvent {
                version: EVENT_VERSION,
                auction_type: 1,
                seller,
                end_time: now + duration,
            },
        );

        auction_id
    }

    /// Create a sealed-bid auction.
    ///
    /// All bids are stored; the highest bidder at or above `reserve_price`
    /// wins when `settle_auction` is called after `end_time`.
    pub fn create_sealed_bid_auction(
        env: Env,
        seller: Address,
        asset_id: u64,
        amount: i128,
        reserve_price: i128,
        duration: u64,
    ) -> u64 {
        seller.require_auth();
        Self::validate_create(amount, reserve_price, reserve_price, duration);

        let auction_id = Self::next_id(&env);
        let now = env.ledger().timestamp();
        let auction = Auction {
            auction_id,
            auction_type: AuctionType::SealedBid,
            seller: seller.clone(),
            asset_id,
            amount,
            reserve_price,
            start_time: now,
            end_time: now + duration,
            status: AuctionStatus::Active,
            start_price: reserve_price,
            highest_bid: 0,
            highest_bidder: None,
            min_bid_increment: 0,
            price_decrement: 0,
            decrement_interval: 0,
            extension_window: 0,
        };

        env.storage()
            .persistent()
            .set(&AuctionDataKey::Auction(auction_id), &auction);

        env.events().publish(
            (
                Symbol::new(&env, "auction_created"),
                auction_id,
                asset_id,
            ),
            AuctionCreatedEvent {
                version: EVENT_VERSION,
                auction_type: 2,
                seller,
                end_time: now + duration,
            },
        );

        auction_id
    }

    // -----------------------------------------------------------------------
    // Bidding
    // -----------------------------------------------------------------------

    /// Place a bid on an English or SealedBid auction.
    ///
    /// For English auctions:
    ///   - `amount` must beat the current highest bid by at least `min_bid_increment`.
    ///   - The previous highest bidder's amount is recorded as a pending refund.
    ///   - If the bid arrives within `extension_window` seconds of `end_time`,
    ///     the deadline is extended by `extension_window` to prevent sniping.
    ///
    /// For SealedBid auctions:
    ///   - Each bidder may only bid once; a higher bid replaces the previous one
    ///     and the difference is credited as a pending refund.
    pub fn place_bid(env: Env, bidder: Address, auction_id: u64, amount: i128) {
        bidder.require_auth();

        let mut auction: Auction = env
            .storage()
            .persistent()
            .get(&AuctionDataKey::Auction(auction_id))
            .expect("auction not found");

        if auction.status != AuctionStatus::Active {
            panic!("auction is not active");
        }

        let now = env.ledger().timestamp();
        if now >= auction.end_time {
            panic!("auction has ended");
        }

        match auction.auction_type {
            AuctionType::Dutch => panic!("use buy_dutch for Dutch auctions"),
            AuctionType::English => {
                let min_required = if auction.highest_bid == 0 {
                    auction.start_price
                } else {
                    auction
                        .highest_bid
                        .checked_add(auction.min_bid_increment)
                        .expect("overflow")
                };
                if amount < min_required {
                    panic!("bid too low");
                }

                // Record pending refund for the outbid party
                if let Some(prev_bidder) = auction.highest_bidder.clone() {
                    Self::add_pending_refund(&env, auction_id, &prev_bidder, auction.highest_bid);
                    env.events().publish(
                        (Symbol::new(&env, "bid_refunded"), auction_id),
                        BidRefundedEvent {
                            version: EVENT_VERSION,
                            bidder: prev_bidder,
                            amount: auction.highest_bid,
                        },
                    );
                }

                auction.highest_bid = amount;
                auction.highest_bidder = Some(bidder.clone());

                // Anti-sniping: extend deadline if bid arrives near the end
                if auction.extension_window > 0
                    && now + auction.extension_window >= auction.end_time
                {
                    auction.end_time = now + auction.extension_window;
                }
            }
            AuctionType::SealedBid => {
                if amount <= 0 {
                    panic!("bid must be positive");
                }

                // Allow one improved bid per bidder
                if let Some(prev_bid) = env
                    .storage()
                    .persistent()
                    .get::<_, Bid>(&AuctionDataKey::Bid(auction_id, bidder.clone()))
                {
                    if amount <= prev_bid.amount {
                        panic!("new bid must exceed previous bid");
                    }
                    // Refund the difference between old and new (net locking)
                    let refund = amount
                        .checked_sub(prev_bid.amount)
                        .expect("underflow");
                    Self::add_pending_refund(&env, auction_id, &bidder, refund);
                } else {
                    // First bid – track this bidder
                    Self::append_bidder(&env, auction_id, &bidder);
                }

                // Update internal highest bid tracker (used for analytics; settlement
                // will do a full scan to find the true winner)
                if amount > auction.highest_bid {
                    auction.highest_bid = amount;
                    auction.highest_bidder = Some(bidder.clone());
                }
            }
        }

        let bid = Bid {
            bidder: bidder.clone(),
            amount,
            timestamp: now,
        };
        env.storage()
            .persistent()
            .set(&AuctionDataKey::Bid(auction_id, bidder.clone()), &bid);

        env.storage()
            .persistent()
            .set(&AuctionDataKey::Auction(auction_id), &auction);

        env.events().publish(
            (Symbol::new(&env, "bid_placed"), auction_id),
            BidPlacedEvent {
                version: EVENT_VERSION,
                bidder,
                amount,
                timestamp: now,
            },
        );
    }

    /// Immediately purchase the asset in a Dutch auction at the current price.
    ///
    /// The buyer must supply `amount` >= the current calculated price.
    pub fn buy_dutch(env: Env, buyer: Address, auction_id: u64) {
        buyer.require_auth();

        let mut auction: Auction = env
            .storage()
            .persistent()
            .get(&AuctionDataKey::Auction(auction_id))
            .expect("auction not found");

        if auction.status != AuctionStatus::Active {
            panic!("auction is not active");
        }
        if auction.auction_type != AuctionType::Dutch {
            panic!("not a Dutch auction");
        }

        let now = env.ledger().timestamp();
        if now >= auction.end_time {
            panic!("auction has ended");
        }

        let current_price = Self::dutch_price_at(&auction, now);

        auction.highest_bid = current_price;
        auction.highest_bidder = Some(buyer.clone());
        auction.status = AuctionStatus::Settled;

        env.storage()
            .persistent()
            .set(&AuctionDataKey::Auction(auction_id), &auction);

        env.events().publish(
            (
                Symbol::new(&env, "auction_settled"),
                auction_id,
                auction.asset_id,
            ),
            AuctionSettledEvent {
                version: EVENT_VERSION,
                winner: Some(buyer),
                winning_bid: current_price,
                timestamp: now,
            },
        );
    }

    // -----------------------------------------------------------------------
    // Settlement
    // -----------------------------------------------------------------------

    /// Settle an English or SealedBid auction after `end_time` has passed.
    ///
    /// For English auctions: the current `highest_bidder` wins if the bid
    /// meets `reserve_price`.
    ///
    /// For SealedBid auctions: all bids are scanned to find the true winner;
    /// all other bidders have their amounts recorded as pending refunds.
    ///
    /// Emits `auction_settled` with the outcome.
    pub fn settle_auction(env: Env, auction_id: u64) {
        let mut auction: Auction = env
            .storage()
            .persistent()
            .get(&AuctionDataKey::Auction(auction_id))
            .expect("auction not found");

        if auction.status != AuctionStatus::Active {
            panic!("auction already finalized");
        }

        let now = env.ledger().timestamp();
        if now < auction.end_time {
            panic!("auction has not ended yet");
        }

        auction.status = AuctionStatus::Ended;

        let winner: Option<Address>;
        let winning_bid: i128;

        match auction.auction_type {
            AuctionType::Dutch => panic!("Dutch auctions settle via buy_dutch"),
            AuctionType::English => {
                if auction.highest_bid >= auction.reserve_price
                    && auction.highest_bidder.is_some()
                {
                    winner = auction.highest_bidder.clone();
                    winning_bid = auction.highest_bid;
                    auction.status = AuctionStatus::Settled;
                } else {
                    // Reserve not met – refund the highest bidder if any
                    if let Some(ref top_bidder) = auction.highest_bidder {
                        Self::add_pending_refund(
                            &env,
                            auction_id,
                            top_bidder,
                            auction.highest_bid,
                        );
                        env.events().publish(
                            (Symbol::new(&env, "bid_refunded"), auction_id),
                            BidRefundedEvent {
                                version: EVENT_VERSION,
                                bidder: top_bidder.clone(),
                                amount: auction.highest_bid,
                            },
                        );
                    }
                    winner = None;
                    winning_bid = 0;
                    auction.status = AuctionStatus::Ended;
                }
            }
            AuctionType::SealedBid => {
                // Full scan: find the highest bid at or above reserve
                let bidders: Vec<Address> = env
                    .storage()
                    .persistent()
                    .get(&AuctionDataKey::Bidders(auction_id))
                    .unwrap_or(Vec::new(&env));

                let mut best_bidder: Option<Address> = None;
                let mut best_amount: i128 = 0;

                for bidder in bidders.iter() {
                    if let Some(bid) = env
                        .storage()
                        .persistent()
                        .get::<_, Bid>(&AuctionDataKey::Bid(auction_id, bidder.clone()))
                    {
                        if bid.amount > best_amount {
                            best_amount = bid.amount;
                            best_bidder = Some(bidder);
                        }
                    }
                }

                if best_amount >= auction.reserve_price && best_bidder.is_some() {
                    // Refund all losing bidders
                    let winner_addr = best_bidder.clone().unwrap();
                    for bidder in bidders.iter() {
                        if bidder != winner_addr {
                            if let Some(bid) = env
                                .storage()
                                .persistent()
                                .get::<_, Bid>(&AuctionDataKey::Bid(auction_id, bidder.clone()))
                            {
                                Self::add_pending_refund(&env, auction_id, &bidder, bid.amount);
                                env.events().publish(
                                    (Symbol::new(&env, "bid_refunded"), auction_id),
                                    BidRefundedEvent {
                                        version: EVENT_VERSION,
                                        bidder,
                                        amount: bid.amount,
                                    },
                                );
                            }
                        }
                    }
                    winner = best_bidder;
                    winning_bid = best_amount;
                    auction.status = AuctionStatus::Settled;
                } else {
                    // Reserve not met – refund everyone
                    for bidder in bidders.iter() {
                        if let Some(bid) = env
                            .storage()
                            .persistent()
                            .get::<_, Bid>(&AuctionDataKey::Bid(auction_id, bidder.clone()))
                        {
                            Self::add_pending_refund(&env, auction_id, &bidder, bid.amount);
                            env.events().publish(
                                (Symbol::new(&env, "bid_refunded"), auction_id),
                                BidRefundedEvent {
                                    version: EVENT_VERSION,
                                    bidder,
                                    amount: bid.amount,
                                },
                            );
                        }
                    }
                    winner = None;
                    winning_bid = 0;
                    auction.status = AuctionStatus::Ended;
                }
            }
        }

        auction.highest_bidder = winner.clone();
        auction.highest_bid = winning_bid;
        env.storage()
            .persistent()
            .set(&AuctionDataKey::Auction(auction_id), &auction);

        env.events().publish(
            (
                Symbol::new(&env, "auction_settled"),
                auction_id,
                auction.asset_id,
            ),
            AuctionSettledEvent {
                version: EVENT_VERSION,
                winner,
                winning_bid,
                timestamp: now,
            },
        );
    }

    // -----------------------------------------------------------------------
    // Cancellation
    // -----------------------------------------------------------------------

    /// Cancel an active auction.
    ///
    /// Only the seller or the admin may cancel.  All existing bids are
    /// refunded via pending-refund entries and events.
    pub fn cancel_auction(env: Env, actor: Address, auction_id: u64) {
        actor.require_auth();

        let mut auction: Auction = env
            .storage()
            .persistent()
            .get(&AuctionDataKey::Auction(auction_id))
            .expect("auction not found");

        if auction.status != AuctionStatus::Active {
            panic!("only active auctions can be cancelled");
        }

        let admin: Address = env
            .storage()
            .instance()
            .get(&AuctionDataKey::Admin)
            .expect("not initialized");

        if actor != auction.seller && actor != admin {
            panic!("only seller or admin can cancel");
        }

        Self::refund_all_bids(&env, auction_id, &auction);
        auction.status = AuctionStatus::Cancelled;
        env.storage()
            .persistent()
            .set(&AuctionDataKey::Auction(auction_id), &auction);

        let now = env.ledger().timestamp();
        env.events().publish(
            (Symbol::new(&env, "auction_cancelled"), auction_id),
            AuctionCancelledEvent {
                version: EVENT_VERSION,
                cancelled_by: actor,
                timestamp: now,
            },
        );
    }

    /// Emergency cancellation by admin only.  Bypasses seller check and
    /// always succeeds for any active auction.
    pub fn emergency_cancel(env: Env, admin: Address, auction_id: u64) {
        Self::require_admin(&env, &admin);

        let mut auction: Auction = env
            .storage()
            .persistent()
            .get(&AuctionDataKey::Auction(auction_id))
            .expect("auction not found");

        if auction.status != AuctionStatus::Active {
            panic!("only active auctions can be cancelled");
        }

        Self::refund_all_bids(&env, auction_id, &auction);
        auction.status = AuctionStatus::Cancelled;
        env.storage()
            .persistent()
            .set(&AuctionDataKey::Auction(auction_id), &auction);

        let now = env.ledger().timestamp();
        env.events().publish(
            (Symbol::new(&env, "auction_cancelled"), auction_id),
            AuctionCancelledEvent {
                version: EVENT_VERSION,
                cancelled_by: admin,
                timestamp: now,
            },
        );
    }

    // -----------------------------------------------------------------------
    // Queries
    // -----------------------------------------------------------------------

    pub fn get_auction(env: Env, auction_id: u64) -> Option<Auction> {
        env.storage()
            .persistent()
            .get(&AuctionDataKey::Auction(auction_id))
    }

    /// Return the current price for a Dutch auction at the current ledger time.
    pub fn get_dutch_current_price(env: Env, auction_id: u64) -> i128 {
        let auction: Auction = env
            .storage()
            .persistent()
            .get(&AuctionDataKey::Auction(auction_id))
            .expect("auction not found");
        if auction.auction_type != AuctionType::Dutch {
            panic!("not a Dutch auction");
        }
        let now = env.ledger().timestamp();
        Self::dutch_price_at(&auction, now)
    }

    pub fn get_bid(env: Env, auction_id: u64, bidder: Address) -> Option<Bid> {
        env.storage()
            .persistent()
            .get(&AuctionDataKey::Bid(auction_id, bidder))
    }

    pub fn get_bidders(env: Env, auction_id: u64) -> Vec<Address> {
        env.storage()
            .persistent()
            .get(&AuctionDataKey::Bidders(auction_id))
            .unwrap_or(Vec::new(&env))
    }

    pub fn get_pending_refund(env: Env, auction_id: u64, bidder: Address) -> i128 {
        env.storage()
            .persistent()
            .get(&AuctionDataKey::PendingRefund(auction_id, bidder))
            .unwrap_or(0)
    }

    // -----------------------------------------------------------------------
    // Internal helpers
    // -----------------------------------------------------------------------

    fn require_admin(env: &Env, caller: &Address) {
        let admin: Address = env
            .storage()
            .instance()
            .get(&AuctionDataKey::Admin)
            .expect("not initialized");
        if *caller != admin {
            panic!("admin only");
        }
        caller.require_auth();
    }

    fn next_id(env: &Env) -> u64 {
        let id: u64 = env
            .storage()
            .instance()
            .get(&AuctionDataKey::AuctionNonce)
            .unwrap_or(0)
            + 1;
        env.storage()
            .instance()
            .set(&AuctionDataKey::AuctionNonce, &id);
        id
    }

    fn validate_create(amount: i128, start_price: i128, reserve_price: i128, duration: u64) {
        if amount <= 0 {
            panic!("amount must be positive");
        }
        if start_price <= 0 {
            panic!("start_price must be positive");
        }
        if reserve_price < 0 {
            panic!("reserve_price must be non-negative");
        }
        if duration == 0 {
            panic!("duration must be non-zero");
        }
    }

    /// Calculate the current Dutch auction price, clamped at `reserve_price`.
    fn dutch_price_at(auction: &Auction, now: u64) -> i128 {
        let elapsed = now.saturating_sub(auction.start_time);
        let steps = elapsed / auction.decrement_interval;
        let drop = auction
            .price_decrement
            .checked_mul(steps as i128)
            .unwrap_or(i128::MAX);
        let price = auction.start_price.saturating_sub(drop);
        price.max(auction.reserve_price)
    }

    fn add_pending_refund(env: &Env, auction_id: u64, bidder: &Address, amount: i128) {
        let key = AuctionDataKey::PendingRefund(auction_id, bidder.clone());
        let existing: i128 = env.storage().persistent().get(&key).unwrap_or(0);
        env.storage()
            .persistent()
            .set(&key, &(existing + amount));
    }

    fn append_bidder(env: &Env, auction_id: u64, bidder: &Address) {
        let key = AuctionDataKey::Bidders(auction_id);
        let mut list: Vec<Address> = env
            .storage()
            .persistent()
            .get(&key)
            .unwrap_or(Vec::new(env));
        list.push_back(bidder.clone());
        env.storage().persistent().set(&key, &list);
    }

    fn refund_all_bids(env: &Env, auction_id: u64, auction: &Auction) {
        // English/Dutch: single highest bidder
        if auction.auction_type != AuctionType::SealedBid {
            if let Some(ref b) = auction.highest_bidder {
                Self::add_pending_refund(env, auction_id, b, auction.highest_bid);
                env.events().publish(
                    (Symbol::new(env, "bid_refunded"), auction_id),
                    BidRefundedEvent {
                        version: EVENT_VERSION,
                        bidder: b.clone(),
                        amount: auction.highest_bid,
                    },
                );
            }
            return;
        }

        // SealedBid: all bidders
        let bidders: Vec<Address> = env
            .storage()
            .persistent()
            .get(&AuctionDataKey::Bidders(auction_id))
            .unwrap_or(Vec::new(env));
        for bidder in bidders.iter() {
            if let Some(bid) = env
                .storage()
                .persistent()
                .get::<_, Bid>(&AuctionDataKey::Bid(auction_id, bidder.clone()))
            {
                Self::add_pending_refund(env, auction_id, &bidder, bid.amount);
                env.events().publish(
                    (Symbol::new(env, "bid_refunded"), auction_id),
                    BidRefundedEvent {
                        version: EVENT_VERSION,
                        bidder,
                        amount: bid.amount,
                    },
                );
            }
        }
    }
}

// ===========================================================================
// Unit Tests
// ===========================================================================

#[cfg(test)]
mod test {
    use super::*;
    use soroban_sdk::testutils::{Address as _, Ledger};

    fn setup() -> (Env, Address, Address) {
        let env = Env::default();
        env.mock_all_auths();
        let admin = Address::generate(&env);
        let contract_id = env.register_contract(None, AuctionHouse);
        let client = AuctionHouseClient::new(&env, &contract_id);
        client.initialize(&admin);
        (env, contract_id, admin)
    }

    // -----------------------------------------------------------------------
    // English auction
    // -----------------------------------------------------------------------

    #[test]
    fn test_english_auction_basic_flow() {
        let (env, id, _admin) = setup();
        let client = AuctionHouseClient::new(&env, &id);

        let seller = Address::generate(&env);
        let bidder1 = Address::generate(&env);
        let bidder2 = Address::generate(&env);

        let auction_id = client.create_english_auction(
            &seller, &1, &100, &50, &50, &10, &3600, &60,
        );

        // First bid
        client.place_bid(&bidder1, &auction_id, &60);
        let a = client.get_auction(&auction_id).unwrap();
        assert_eq!(a.highest_bid, 60);

        // Outbid: must beat 60 + 10 = 70
        client.place_bid(&bidder2, &auction_id, &80);
        let a = client.get_auction(&auction_id).unwrap();
        assert_eq!(a.highest_bid, 80);

        // bidder1 has a pending refund of 60
        assert_eq!(client.get_pending_refund(&auction_id, &bidder1), 60);

        // Settle after end
        env.ledger().with_mut(|li| li.timestamp += 3601);
        client.settle_auction(&auction_id);

        let a = client.get_auction(&auction_id).unwrap();
        assert_eq!(a.status, AuctionStatus::Settled);
        assert_eq!(a.highest_bidder, Some(bidder2));
    }

    #[test]
    #[should_panic(expected = "bid too low")]
    fn test_english_bid_too_low_panics() {
        let (env, id, _admin) = setup();
        let client = AuctionHouseClient::new(&env, &id);

        let seller = Address::generate(&env);
        let bidder = Address::generate(&env);

        let auction_id = client.create_english_auction(
            &seller, &1, &100, &100, &50, &20, &3600, &0,
        );

        client.place_bid(&bidder, &auction_id, &50); // below start_price=100
    }

    #[test]
    fn test_english_auction_extension_on_late_bid() {
        let (env, id, _admin) = setup();
        let client = AuctionHouseClient::new(&env, &id);

        let seller = Address::generate(&env);
        let bidder = Address::generate(&env);

        // duration=3600, extension_window=300
        let auction_id = client.create_english_auction(
            &seller, &1, &100, &50, &50, &10, &3600, &300,
        );

        // Jump to 50 seconds before end
        env.ledger().with_mut(|li| li.timestamp += 3550);

        let before = client.get_auction(&auction_id).unwrap().end_time;
        client.place_bid(&bidder, &auction_id, &60);
        let after = client.get_auction(&auction_id).unwrap().end_time;

        assert!(after > before, "end_time should be extended");
    }

    #[test]
    fn test_english_reserve_not_met_refunds_bidder() {
        let (env, id, _admin) = setup();
        let client = AuctionHouseClient::new(&env, &id);

        let seller = Address::generate(&env);
        let bidder = Address::generate(&env);

        // start_price=50 (opening bid floor), reserve_price=200 (minimum to settle)
        // bid of 60 clears the opening floor but falls far below the reserve
        let auction_id = client.create_english_auction(
            &seller, &1, &100, &50, &200, &10, &3600, &0,
        );

        client.place_bid(&bidder, &auction_id, &60);
        env.ledger().with_mut(|li| li.timestamp += 3601);
        client.settle_auction(&auction_id);

        let a = client.get_auction(&auction_id).unwrap();
        assert_eq!(a.status, AuctionStatus::Ended);
        assert_eq!(client.get_pending_refund(&auction_id, &bidder), 60);
    }

    // -----------------------------------------------------------------------
    // Dutch auction
    // -----------------------------------------------------------------------

    #[test]
    fn test_dutch_auction_buy_at_current_price() {
        let (env, id, _admin) = setup();
        let client = AuctionHouseClient::new(&env, &id);

        let seller = Address::generate(&env);
        let buyer = Address::generate(&env);

        // start=1000, drops by 10 every 100s, reserve=500
        let auction_id = client.create_dutch_auction(
            &seller, &2, &50, &1000, &500, &10, &100, &7200,
        );

        // Advance 300 seconds → price = 1000 - 3*10 = 970
        env.ledger().with_mut(|li| li.timestamp += 300);
        let price = client.get_dutch_current_price(&auction_id);
        assert_eq!(price, 970);

        client.buy_dutch(&buyer, &auction_id);

        let a = client.get_auction(&auction_id).unwrap();
        assert_eq!(a.status, AuctionStatus::Settled);
        assert_eq!(a.highest_bidder, Some(buyer));
        assert_eq!(a.highest_bid, 970);
    }

    #[test]
    fn test_dutch_price_clamped_at_reserve() {
        let (env, id, _admin) = setup();
        let client = AuctionHouseClient::new(&env, &id);

        let seller = Address::generate(&env);

        let auction_id = client.create_dutch_auction(
            &seller, &3, &10, &100, &50, &10, &10, &7200,
        );

        // Advance 1000s → drops would be 1000/10 * 10 = 1000, but clamped at reserve=50
        env.ledger().with_mut(|li| li.timestamp += 1000);
        let price = client.get_dutch_current_price(&auction_id);
        assert_eq!(price, 50);
    }

    // -----------------------------------------------------------------------
    // Sealed-bid auction
    // -----------------------------------------------------------------------

    #[test]
    fn test_sealed_bid_winner_selected_correctly() {
        let (env, id, _admin) = setup();
        let client = AuctionHouseClient::new(&env, &id);

        let seller = Address::generate(&env);
        let b1 = Address::generate(&env);
        let b2 = Address::generate(&env);
        let b3 = Address::generate(&env);

        let auction_id = client.create_sealed_bid_auction(&seller, &4, &1, &50, &3600);

        client.place_bid(&b1, &auction_id, &70);
        client.place_bid(&b2, &auction_id, &120);
        client.place_bid(&b3, &auction_id, &90);

        env.ledger().with_mut(|li| li.timestamp += 3601);
        client.settle_auction(&auction_id);

        let a = client.get_auction(&auction_id).unwrap();
        assert_eq!(a.status, AuctionStatus::Settled);
        assert_eq!(a.highest_bidder, Some(b2.clone()));

        // Losers should have pending refunds
        assert_eq!(client.get_pending_refund(&auction_id, &b1), 70);
        assert_eq!(client.get_pending_refund(&auction_id, &b3), 90);
        // Winner has no refund
        assert_eq!(client.get_pending_refund(&auction_id, &b2), 0);
    }

    // -----------------------------------------------------------------------
    // Cancellation
    // -----------------------------------------------------------------------

    #[test]
    fn test_cancel_auction_by_seller() {
        let (env, id, _admin) = setup();
        let client = AuctionHouseClient::new(&env, &id);

        let seller = Address::generate(&env);
        let auction_id = client.create_english_auction(
            &seller, &1, &100, &50, &50, &10, &3600, &0,
        );

        client.cancel_auction(&seller, &auction_id);

        let a = client.get_auction(&auction_id).unwrap();
        assert_eq!(a.status, AuctionStatus::Cancelled);
    }

    #[test]
    fn test_emergency_cancel_by_admin() {
        let (env, id, admin) = setup();
        let client = AuctionHouseClient::new(&env, &id);

        let seller = Address::generate(&env);
        let bidder = Address::generate(&env);

        let auction_id = client.create_english_auction(
            &seller, &1, &100, &50, &50, &10, &3600, &0,
        );
        client.place_bid(&bidder, &auction_id, &100);

        client.emergency_cancel(&admin, &auction_id);

        let a = client.get_auction(&auction_id).unwrap();
        assert_eq!(a.status, AuctionStatus::Cancelled);
        assert_eq!(client.get_pending_refund(&auction_id, &bidder), 100);
    }

    #[test]
    #[should_panic(expected = "only seller or admin can cancel")]
    fn test_cancel_by_unauthorized_panics() {
        let (env, id, _admin) = setup();
        let client = AuctionHouseClient::new(&env, &id);

        let seller = Address::generate(&env);
        let rando = Address::generate(&env);

        let auction_id = client.create_english_auction(
            &seller, &1, &100, &50, &50, &10, &3600, &0,
        );
        client.cancel_auction(&rando, &auction_id);
    }

    // -----------------------------------------------------------------------
    // Double-initialization
    // -----------------------------------------------------------------------

    #[test]
    #[should_panic(expected = "already initialized")]
    fn test_double_initialize_panics() {
        let (env, id, admin) = setup();
        let client = AuctionHouseClient::new(&env, &id);
        client.initialize(&admin);
    }
}
