# Automated Dividend Distribution System

## Overview

The AssetForge contract now includes an automated dividend distribution system with snapshot-based allocation, proportional distribution, claim tracking, and tax withholding support.

## Features

### 1. Snapshot Mechanism
- **create_snapshot**: Creates a snapshot of token holders at a specific timestamp
- **get_snapshot**: Retrieves snapshot information including timestamp, total supply, and holder count
- Snapshots are used to determine dividend eligibility based on token holdings at distribution time

### 2. Dividend Distribution
- **create_dividend_distribution**: Creates a new dividend distribution with:
  - Snapshot-based allocation (uses existing snapshot or creates new one)
  - Configurable tax withholding rate (in basis points)
  - Multiple asset support (any payout asset)
  - Distribution history tracking
  - Emergency pause capability

### 3. Claim System
- **claim_dividend_distribution**: Allows token holders to claim their dividend share
  - Proportional distribution based on snapshot balance
  - Automatic tax withholding calculation
  - Prevents double-claiming
  - Requires token ownership at snapshot time

### 4. Tax Withholding
- Configurable tax rate (0-10000 basis points, where 10000 = 100%)
- Calculated as: `withheld = gross_amount * tax_rate / 10000`
- Net payout: `net_amount = gross_amount - withheld`
- Tracked per claim for audit purposes

### 5. Emergency Controls
- **pause_dividends**: Pauses all dividend distributions globally
- **resume_dividends**: Resumes global dividend operations
- **pause_distribution**: Pauses a specific distribution
- **resume_distribution**: Resumes a specific distribution
- **are_dividends_paused**: Checks global pause status

### 6. Distribution History
- **get_distribution_history**: Retrieves all distribution IDs for an asset
- **get_distribution**: Retrieves detailed distribution information
- **get_dividend_claim**: Retrieves claim information for a specific distribution and claimant

### 7. Unclaimed Dividends
- **calculate_unclaimed_dividends**: Calculates remaining unclaimed amount for a distribution
- Useful for tracking distribution completion and fund management

## Data Structures

### DividendDistribution
```rust
pub struct DividendDistribution {
    pub distribution_id: u64,
    pub asset_id: u64,
    pub total_amount: i128,
    pub payout_asset: Address,
    pub timestamp: u64,
    pub snapshot_timestamp: u64,
    pub total_supply: i128,
    pub tax_withholding_rate: u32, // basis points
    pub is_paused: bool,
}
```

### DividendClaim
```rust
pub struct DividendClaim {
    pub distribution_id: u64,
    pub claimant: Address,
    pub amount: i128,      // net amount after tax
    pub withheld: i128,    // tax withheld
    pub claimed_at: u64,
}
```

### TokenSnapshot
```rust
pub struct TokenSnapshot {
    pub snapshot_id: u64,
    pub timestamp: u64,
    pub total_supply: i128,
    pub holder_count: u32,
}
```

## Usage Example

### Creating a Distribution
```rust
// 1. Create a snapshot (optional - can be done automatically)
let snapshot_id = client.create_snapshot(&admin);

// 2. Create dividend distribution with 5% tax withholding
let payout_asset = Address::generate(&env);
let distribution_id = client.create_dividend_distribution(
    &admin, 
    &1,              // asset_id
    &100000,         // total_amount
    &payout_asset,   // payout_asset
    &500,            // tax_withholding_rate (5% = 500 bps)
    &Some(snapshot_id)
);
```

### Claiming Dividends
```rust
// User claims their dividend share
client.claim_dividend_distribution(&distribution_id, &user);

// Check claim details
let claim = client.get_dividend_claim(&distribution_id, &user).unwrap();
println!("Net amount: {}", claim.amount);
println!("Tax withheld: {}", claim.withheld);
```

### Emergency Pause
```rust
// Pause all distributions
client.pause_dividends(&admin);

// Pause specific distribution
client.pause_distribution(&admin, &distribution_id);

// Resume operations
client.resume_dividends(&admin);
client.resume_distribution(&admin, &distribution_id);
```

## Gas Efficiency

The system is designed for gas efficiency:
- Snapshots are created once and reused for multiple distributions
- Claim tracking uses indexed storage for quick lookups
- Distribution history is stored per asset to minimize storage costs
- Tax calculations are done on-chain with simple arithmetic

## Security Considerations

1. **Admin Authorization**: All distribution creation and pause functions require admin authorization
2. **Claim Prevention**: Double-claiming is prevented through storage checks
3. **Pause Mechanism**: Emergency pause allows quick response to issues
4. **Tax Withholding**: Configurable tax rates support regulatory compliance
5. **Snapshot Integrity**: Snapshots are immutable once created

## Testing

Comprehensive tests cover:
- Snapshot creation and retrieval
- Distribution creation with various tax rates
- Claim functionality and double-claim prevention
- Emergency pause and resume operations
- Distribution history tracking
- Tax withholding calculations
- Unclaimed dividend calculations

Run tests with:
```bash
cargo test --lib dividend
cargo test --lib snapshot
cargo test --lib distribution
```

## Future Enhancements

Potential improvements:
- Automated snapshot scheduling
- Multi-asset distribution support
- Claim deadline enforcement
- Vesting schedules for dividends
- Delegated claiming
- Off-chain claim aggregation for gas optimization
